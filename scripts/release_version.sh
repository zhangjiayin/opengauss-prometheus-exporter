#!/bin/bash

# -----------------------------------------------------------------------------
# Copyright (c) 2020-2021 Yunhe Enmo (Beijing) Information Technology Co., Ltd
#
# mtk is licensed under Mulan PSL v2.
# You can use this software according to the terms and conditions of the Mulan PSL v2.
# You may obtain a copy of Mulan PSL v2 at:
#
#          http://license.coscl.org.cn/MulanPSL2
#
# THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
# EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
# MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
# See the Mulan PSL v2 for more details.
# -----------------------------------------------------------------------------
# Author        : Bin Liu
# File          : release_version.sh
# -----------------------------------------------------------------------------

VERSION_FILE="pkg/version/version.go"
VERSION=`grep 'var version' ${VERSION_FILE} | sed -E 's/.*"(.+)"$$/v\1/'`



function _usage()
{
  ###### U S A G E : Help and ERROR ######
  cat <<EOF
  $0 $Options
  $*
          Usage: $0 <[options]>
          Options:
                  -p --major update major version
                  -s --minor update minor version
                  -b --bump  only bump version not tag
EOF
  exit 0
}


function _cleanup ()
{
  unset -f _usage _cleanup ; return 0
}

## Clear out nested functions on exit
trap _cleanup INT EXIT RETURN

###### some declarations for this example ######
Options=$@
Optnum=$#
update_major='no'
update_minor='no'
bump_version='no'
##################################################################
#######  "getopts" with: short options  AND  long options  #######
#######            AND  short/long arguments               #######
while getopts ':pshb' OPTION ; do
  case "$OPTION" in
    p  ) update_major=yes                       ;;
    s  ) update_minor=yes                       ;;
    h  ) _usage                                 ;;
    b  ) bump_version=yes                       ;;
    -  ) [ $OPTIND -ge 1 ] && optind=$(expr $OPTIND - 1 ) || optind=$OPTIND
         eval OPTION="\$$optind"
         OPTARG=$(echo $OPTION | cut -d'=' -f2)
         OPTION=$(echo $OPTION | cut -d'=' -f1)
         case $OPTION in
             --major       ) update_major=yes                       ;;
             --minor       ) update_minor=yes                       ;;
             --help        ) _usage                                 ;;
             --bump        ) bump_version=yes                       ;;
             * )  _usage " Long: >>>>>>>> invalid options (long) "  ;;
         esac
       OPTIND=1
       shift
      ;;
    ? )  _usage "Short: >>>>>>>> invalid options (short) "          ;;
  esac
done

echo "Now  Version ${VERSION}"
version_str=${VERSION#*v}
MAJOR=`echo $version_str | cut -d '.' -f 1`
MINOR=`echo $version_str| cut -d '.' -f 2`
PATCH=`echo $version_str| cut -d '.' -f 3`

if [[ "${update_major}" == "yes" ]]; then
  MAJOR=`expr ${MAJOR} + 1`
  MINOR=0
  PATCH=0
elif [[ "${update_minor}" == "yes" ]]; then
  MINOR=`expr ${MINOR} + 1`
  PATCH=0
else
  PATCH=`expr ${PATCH} + 1`
fi

version_str_new="${MAJOR}.${MINOR}.${PATCH}"

if [[ "${bump_version}" == "no" ]]; then
  echo "Release ${VERSION}"
  git tag ${VERSION}
  make changelog
  git add CHANGELOG.md
  git commit -m "release version ${VERSION}"
  git tag -d ${VERSION}
  git tag ${VERSION}
fi
#

echo "Bump Version v${version_str_new}"
sed -i "s/version = \"${version_str}\"/version = \"${version_str_new}\"/g" ${VERSION_FILE}
sed -i "s/BETA_VERSION=${VERSION}/BETA_VERSION=v${version_str_new}/g" .goreleaser.yml

git add ${VERSION_FILE} .goreleaser.yml
git commit -m "bump version"