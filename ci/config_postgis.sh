#!/bin/bash

set -ex

#	fetch our test data and import it into Postgres. 
#	this command uses pg_restore and therefore leverages the environment variables document at https://www.postgresql.org/docs/9.2/static/libpq-envars.html
configure_postgis() {
    local test_data="tegola.backup"
    local test_data_url="https://s3-us-west-1.amazonaws.com/tegola-test-data/tegola-postgis-test-data.backup"

    #   fetch our test data
    curl $test_data_url > $test_data

    #   import the data to postgres
    pg_restore -C -d postgres $test_data

    #   clean up our test data
    rm $test_data
}

configure_postgis