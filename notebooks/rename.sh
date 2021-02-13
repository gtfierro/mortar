find . -type f -name '*.csv' | while read FILE ; do
    newfile="$(echo ${FILE} |sed -e 's/BRIG/BLDG1/g')" ;
    mv "${FILE}" "${newfile}" ;
done 
