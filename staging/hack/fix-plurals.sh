#!/bin/bash

#
# Script to work around https://github.com/operator-framework/operator-sdk/issues/4453
#

set -e

# allowing sed binary customization, so macos users can use an alternative gnu/sed, usually named "gsed"
SED_BIN="${SED_BIN:-sed}"

${SED_BIN} -i 's/koupletbuildren/koupletbuilds/' config/crd/bases/api.kouplet.com_koupletbuildren.yaml
mv -f config/crd/bases/api.kouplet.com_koupletbuildren.yaml config/crd/bases/api.kouplet.com_koupletbuilds.yaml
