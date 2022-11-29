#!/bin/bash

set -ex

ssh -L 10002:127.0.0.1:26657 gos-gopher
