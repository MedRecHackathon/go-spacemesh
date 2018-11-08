container=${1:-spacemesh}
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $container