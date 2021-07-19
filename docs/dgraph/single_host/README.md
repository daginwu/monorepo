# Dgraph Single Host

## Install 
``` bash
curl https://get.dgraph.io -sSf | bash

# Test that it worked fine, by running:
dgraph
```

### Run Dgraph Zero 
``` bash
dgraph zero --my=IPADDR:5080 &
```

### Run Dgraph Alpha
``` bash
dgraph alpha --my=IPADDR:7080 --zero=localhost:5080 &
dgraph alpha --my=IPADDR:7081 --zero=localhost:5080 -o=1 &
```

### Run Dgraph’s Ratel UI
``` bash
dgraph-ratel
```