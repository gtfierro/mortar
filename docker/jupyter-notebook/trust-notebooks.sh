#!/bin/bash

for nb in `ls *.ipynb` ; do
    jupyter trust $nb
done
