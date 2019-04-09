#!/bin/bash

set -ex

# fetch our test data and import it into Postgres.
# this command uses pg_restore and therefore leverages the environment variables document at https://www.postgresql.org/docs/9.2/static/libpq-envars.html
configure_postgis() {
    local test_data_url="https://raw.githubusercontent.com/go-spatial/tegola-testdata/master/tegola.backup"
    local test_data="tegola.backup"

    # fetch our test data
    curl $test_data_url > $test_data


    # import the data to postgres
    psql -d postgres -c 'DROP DATABASE IF EXISTS "tegola"'

    pg_restore -C -d postgres $test_data

    # used for testing no access queries
    psql -d postgres -c 'CREATE ROLE tegola_no_access LOGIN'

    # clean up our test data
    rm $test_data
}

configure_postgis
