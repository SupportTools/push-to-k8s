#!/bin/bash

setup() {
  while getopts ":s:n:l:fh" opt; do
  case $opt in
    s)
      SLEEP="-s ${OPTARG}"
      ;;
    n)
      SYNCNAMESPACE="-n ${OPTARG}"
      ;;
    l)
      LABELSELECTOR="-l ${OPTARG}"
      ;;
    h)
      help && exit 0
      ;;
    :)
      techo "Option -$OPTARG requires an argument."
      exit 1
      ;;
    *)
      help && exit 0
  esac
  done

  if [[ -z $SLEEP ]]
  then
    SLEEP=360
  fi
  if [[ -z $SYNCNAMESPACE ]]
  then
    SYNCNAMESPACE="push-to-k8s"
  fi
  if [[ -z $LABELSELECTOR ]]
  then
    LABELSELECTOR="exclude"
  else
    if [[ ! $LABELSELECTOR == "exclude" ]] && [[ ! $LABELSELECTOR == "include" ]]
    then
       echo "Need to set the label selector to exclude or include"
       exit 1
    fi
  fi
}

setup-tmp-dir() {
  TMPDIR=$(mktemp -d /tmp/push-to-k8s.XXX)
  if [[ ! -d $TMPDIR ]]
  then
    echo "CRITICAL: Creating TMPDIR"
    exit 2
  else
    echo "Created ${TMPDIR}"
  fi
}

cleanup-tmp-dir() {
  echo "Cleaning up TMPDIR"
  rm -rf ${TMPDIR}
  if [[ -d $TMPDIR ]]
  then
    echo "CRITICAL: TMPDIR wasn't deleted"
    exit 2
  fi
}

get-source-secret() {
  kubectl -n $SYNCNAMESPACE get secret -l push-to-k8s=source -o yaml | grep -v 'push-to-k8s: source' | grep -v 'namespace:' | grep -v 'uid:' | grep -v 'resourceVersion:' > ${TMPDIR}/secret.yaml
}

get-source-configmap() {
  kubectl -n $SYNCNAMESPACE get configmap -l push-to-k8s=source -o yaml | grep -v 'push-to-k8s: source' | grep -v 'namespace:' | grep -v 'uid:' | grep -v 'resourceVersion:' > ${TMPDIR}/configmap.yaml
}

build-source-yaml() {
  echo "Getting source yamls..."
  get-source-secret
  get-source-configmap
}

get-namespaces() {
    if [[ $LABELSELECTOR == "exclude" ]]
    then
      echo "Excluding namespaces using label push-to-k8s"
      namespaces=`kubectl get namespace --selector='!push-to-k8s' -o name | awk -F '/' '{print $2}'`
    else
      echo "Including namespaces using label push-to-k8s"
      namespaces=`kubectl get namespace --selector='push-to-k8s' -o name | awk -F '/' '{print $2}'`
    fi
}

setup
while true
do
  setup-tmp-dir
  build-source-yaml
  get-namespaces
  for namespace in $namespaces
  do
    echo "Namespace: $namespace"
    if [[ $namespace == $SYNCNAMESPACE ]]
    then
      echo "Skipping source namespace"
    else
      echo "Pushing out YAML"
      kubectl -n $namespace apply -f ${TMPDIR}/
    fi
  done
  cleanup-tmp-dir
  sleep ${SLEEP}
done