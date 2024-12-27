#!/bin/bash

export TF_VAR_circle_workflow_id=$CIRCLE_WORKFLOW_ID

terragrunt init \
    -backend-config=access_key=$TF_VAR_terraform_state_bucket_key \
    -backend-config=secret_key=$TF_VAR_terraform_state_bucket_secret \
    -backend-config=region=$TF_VAR_terraform_state_bucket_region

terragrunt apply -auto-approve
