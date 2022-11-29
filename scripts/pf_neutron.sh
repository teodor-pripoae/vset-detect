#!/bin/bash

set -ex

ssh -L 10003:127.0.0.1:26657 gos-neutron
