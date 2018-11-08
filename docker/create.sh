# docker run --name spacemesh -p 8545:8545 spacemesh
# docker build -t spacemesh . && docker run -p 8545:8545 -dit spacemesh

docker rm spacemesh
docker run -p 8545:8545 --name spacemesh -it spacemesh /bin/bash -l
