# Manually building docker image.

  To generate the Dockerfile manually:
run the `generate_dockerfile.sh` . There are a few environmental variables you can set to change the generated Docker file. 

*  VERSION_TAG  sets the version of the docker image. If the variable is not set it will default o the current git commit hash.
* CONTAINER_MAINTAINER sets the MAINTAINER of the image. If the variable is not set; it will default to the current maintainer of the Tegola project.
