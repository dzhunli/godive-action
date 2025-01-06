#!/bin/bash

set -e

IMAGE_NAME=$1
CI_CONFIG=$2
ALLOW_LARGE_IMAGE=$3
CONTINUE_ON_FAIL=$4

check_image_size() {
    IMAGE_SIZE=$(docker image inspect "$1" --format='{{.Size}}')
    IMAGE_SIZE_GB=$(bc <<< "scale=2; $IMAGE_SIZE / 1024 / 1024 / 1024")
    echo -e -n "Image size:"
    echo -e -n "\033[1;33m ${IMAGE_SIZE_GB} \033[0m"
    echo -e "GB"

    if (( $(echo "$IMAGE_SIZE_GB > 1" | bc -l) )); then
        if [[ "$ALLOW_LARGE_IMAGE" != "true" ]]; then
	        echo -e "\033[1;31m Error: The image size exceeds 1 GB. \033[0m"
		echo -e "\n\n"
		echo "#		Pass 'allow_large_image=true' to proceed."
		if [[ "$CONTINUE_ON_FAIL" = "true" ]]; then
        		echo "#         Pass 'continue_on_fail=false' to fail actions that don't pass the test."
			echo -e "\033[1;33m CONTINUE POLICY ENABLED... \033[0m"
			exit 0
		else
			exit 1
		fi
	else
	        echo -e "\033[1;32m Large image allowed. Continuing... \033[0m"
	fi
    fi
}

echo "Checking Docker image size..."
check_image_size "$IMAGE_NAME"

echo "::group::Fetching the latest Dive version..." 
DIVE_VERSION=$(curl -sL "https://api.github.com/repos/wagoodman/dive/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
echo "Latest Dive version: $DIVE_VERSION"
echo "::endgroup::"

echo "::group::Downloading and installing Dive..." 
curl -OL https://github.com/wagoodman/dive/releases/download/v${DIVE_VERSION}/dive_${DIVE_VERSION}_linux_amd64.deb
sudo apt install -qqq ./dive_${DIVE_VERSION}_linux_amd64.deb
echo "::endgroup::"            

echo -e "\033[1;33m Running Dive analysis on image: $IMAGE_NAME with CI config: $CI_CONFIG \033[0m"
if [[ "$CONTINUE_ON_FAIL" = "true" ]]; then
	CI=true dive --ci-config "$CI_CONFIG" "$IMAGE_NAME" || echo -e "\033[1;33m CONTINUE POLICY ENABLED... \033[0m"
	echo -e "\n\n"
	echo "#         Pass 'continue_on_fail=false' to fail actions that don't pass the test."
else
	CI=true dive --ci-config "$CI_CONFIG" "$IMAGE_NAME"
fi
