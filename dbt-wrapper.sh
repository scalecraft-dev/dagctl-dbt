#!/bin/sh
# Wrapper script to install dbt-duckdb and execute dbt commands

echo "Installing dbt-duckdb..."
pip install --quiet dbt-duckdb==1.7.0

echo "Running dbt command: $@"
exec python -m dbt "$@"
