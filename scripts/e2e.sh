#!/bin/sh

DOCKER="docker"
PSQL_CONTAINER="sippy-e2e-test-postgresql"
PSQL_PORT="23433"

if [[ -z "$GCS_SA_JSON_PATH" ]]; then
    echo "Must provide path to GCS credential in GCS_SA_JSON_PATH env var" 1>&2
    exit 1
fi


clean_up () {
    ARG=$?
    echo "Killing sippy API child process: $CHILD_PID"
	kill $CHILD_PID
	echo "Tearing down container $PSQL_CONTAINER"
	$DOCKER stop -i $PSQL_CONTAINER
	$DOCKER rm -i $PSQL_CONTAINER
    exit $ARG
}
trap clean_up EXIT

# make sure no old container running
echo "Cleaning up old sippy postgresql container if present"
$DOCKER stop -i $PSQL_CONTAINER
$DOCKER rm -i $PSQL_CONTAINER

# start postgresql in a container:
echo "Starting new sippy postgresql container: $PSQL_CONTAINER"
$DOCKER run --name $PSQL_CONTAINER -e POSTGRES_PASSWORD=password -p $PSQL_PORT:5432 -d quay.io/enterprisedb/postgresql

echo "Wait 5s for postgresql to start..."
sleep 5

echo "Loading database..."
# use an old release here as they have very few job runs and thus import quickly, ~5 minutes
make build
./sippy load --loader prow --loader releases \
  --release 4.7 \
  --database-dsn="postgresql://postgres:password@localhost:$PSQL_PORT/postgres" \
  --mode=ocp \
  --config ./config/openshift.yaml \
  --google-service-account-credential-file $GCS_SA_JSON_PATH

# Spawn sippy server off into a separate process:
(
./sippy serve \
  --listen ":18080" \
  --listen-metrics ":12112" \
  --database-dsn="postgresql://postgres:password@localhost:$PSQL_PORT/postgres" \
  --log-level debug \
  --mode ocp
)&
# store the child process for cleanup
CHILD_PID=$!

# Give it time to start up
echo "Wait 30s for sippy API to start..."
sleep 30

# Run our tests that request against the API:
go test ./test/e2e/ -v

# WARNING: do not place more commands here without addressing return code from go test not being overridden by the cleanup func


