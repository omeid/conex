set -xe

for box in                      \
  "github.com/conex/postgres"   \
  "github.com/conex/mysql"      \
  "github.com/conex/rethinkdb"  \
  "github.com/conex/mongodb"    \
  "github.com/conex/postgres" 
do
  go get -x -v $box
done
