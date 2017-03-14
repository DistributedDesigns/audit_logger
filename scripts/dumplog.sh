#!/bin/bash

NOW=`date "+%Y-%m-%dT%H%M%S"`
OUTDIR=logs
OUTFILE=${OUTDIR}/${NOW}.xml

if [ ! -d ${OUTDIR} ]; then
  echo "Creating output directory: ${OUTDIR}"
  mkdir -p ${OUTDIR}
fi

echo "Creating output file: ${OUTFILE}"

# Log header
printf "<?xml version=\"1.0\"?>\n<log>" >> ${OUTFILE}

# tail removes the warning about using password on CLI.
# tr removes the CR+LF added by mysql between results.
# xargs inteprets the fully-escaped results from --batch
#   as special characters during printing.
docker exec -it mysql-trading \
  mysql --user=mysqladmin --password=mysqladmin audit \
  --batch --disable-column-names -e \
  "Select content From Logs Order By created_at;" \
  | tail -n +2 | tr -d '\r\n' | xargs --null printf >> ${OUTFILE}

# Log footer
printf "\n</log>\n" >> ${OUTFILE}
