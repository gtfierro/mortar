#!/bin/bash

find $1 -type f -name '*.csv' | xargs -L 1 -n 1 -P 4 python load_csv.py
