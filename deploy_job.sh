#!/bin/bash
kubectl delete -f _manifests/local-job.yml || true && kubectl apply -f _manifests/local-job.yml 