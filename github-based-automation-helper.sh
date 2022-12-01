#!/usr/bin/env bash

# HELP!
# Due to the incredible flexibility of SSM, it's difficult for an opensource project to anticipate how-or-what you intend to do with SMM
# The Golang code is structured to expect a contract depicted in the arguments of this shell script
# and the the SSM Automation Document below. This shell script joins hands with a custom automation document.
#
# ################ SSM Automation Document
#  description: |-
#    *Pull a repo, run a command in/on that repo, clean up the directory*
#
#    ---
#    # Want to run an arbitrary command on a git repo checkout?
#    This document uses a pre-shared private key to checkout a repo.
#    ## Working Directory is controlled
#
#    >for safety/security `/var/tmp` is the relative starting point.
#
#  schemaVersion: '0.3'
#  assumeRole: "{{ AutomationAssumeRole }}"
#  parameters:
#    InstanceIds:
#      type: StringList
#      description: "Managed instance resource id (Node Id)"
#    WorkingDirectory:
#      type: String
#      description: "Where to checkout into and run from"
#      default: /var/tmp/gh
#    Repo:
#      type: String
#      description: "Which repo to checkout, just the repo name, not the org."
#    Cmd:
#      type: String
#      description: "The linux command to run, can be anything present on the target host"
#    GetOptions:
#      type: String
#      description: "Can be empty, or have either \"branch:refs/remotes/origin/yourfeaturebranch\" or \"commitID:123\" not both!"
#    GHPrivateKeyPemParamName:
#      type: String
#      description: "A privateKey registered with GitHub on the matching repo"
#    AutomationAssumeRole:
#      type: String
#      description: "(Optional) The ARN of the role that allows Automation to perform the actions on your behalf."
#      default: ""
#  mainSteps:
#    ....
# MORE HELP!
#    From those main steps a custom ssm document is referenced and it uses two existing SSM Agent built-in plugin capabilities
#     1. aws:downloadContent - https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-plugins.html#aws-downloadContent
#        a. sourceType: "Git"
#        b. sourceInfo: ... see doc
#     2. aws:runShellScript - https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-plugins.html#aws-runShellScript

export AUTOMATION_DOC_NAME="$1"
export TARGET_INSTANCE_ID="$2"
export REPO="$3"
export BRANCH="$4"
export GH_PRIVATE_KEY_SSM_PARAM_NAME="$5"
export REPO_CMD="$6"

set -x
aws ssm start-automation-execution --document-name "${AUTOMATION_DOC_NAME}" \
    --document-version "\$DEFAULT" --target-parameter-name InstanceIds \
    --targets '[{"Key":"ParameterValues","Values":["'"$TARGET_INSTANCE_ID"'"]}]' \
    --parameters "Repo=$REPO,Cmd=$REPO_CMD,GetOptions=branch:refs/remotes/$BRANCH,GHPrivateKeyPemParamName=$GH_PRIVATE_KEY_SSM_PARAM_NAME" --max-errors "0" --max-concurrency "1"