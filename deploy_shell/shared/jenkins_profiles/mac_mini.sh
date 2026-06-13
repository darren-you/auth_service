#!/usr/bin/env bash
# shellcheck shell=bash

apply_jenkins_ssh_profile() {
  JENKINS_SSH_HOST="${JENKINS_SSH_HOST:-124.221.158.155}"
  JENKINS_SSH_PORT="${JENKINS_SSH_PORT:-23}"
  JENKINS_SSH_USER="${JENKINS_SSH_USER:-darrenyou}"
  JENKINS_SSH_PASSWORD="${JENKINS_SSH_PASSWORD:-158825}"
  JENKINS_SSH_KEY_PATH="${JENKINS_SSH_KEY_PATH:-}"
  JENKINS_SSH_OPTIONS="${JENKINS_SSH_OPTIONS:--o PreferredAuthentications=password,keyboard-interactive -o PubkeyAuthentication=no}"
  JENKINS_SSH_TARGET="${JENKINS_SSH_TARGET:-${JENKINS_SSH_USER}@${JENKINS_SSH_HOST}}"
}
