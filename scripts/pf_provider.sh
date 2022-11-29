#!/bin/bash

set -ex

ssh -L 10001:127.0.0.1:26657 gos-provider
