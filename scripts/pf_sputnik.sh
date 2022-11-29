#!/bin/bash

set -ex

ssh -L 10004:127.0.0.1:26659 gos-sputnik
