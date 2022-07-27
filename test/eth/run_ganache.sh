CWD="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
DATA_DIR=${DATA_DIR:-$CWD/data}

rm -rf $DATA_DIR
mkdir -p $DATA_DIR

ganache="ganache-cli"

if pgrep -x $ganache >/dev/null
then
  echo "$ganache is running, going to kill all"
  ps -ef | grep $ganache | grep -v grep | awk '{print $2}' | xargs kill
fi

ganache-cli \
  --chain-id 888 \
  --account '0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305,1000000000000000000' \
  --account '0x741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544,1000000000000000000' \
  --account '0x39a4c898dda351d54875d5ebb3e1c451189116faa556c3c04adc860dd1000608,1000000000000000000' \
  --account '0x6c212553111b370a8ffdc682954495b7b90a73cedab7106323646a4f2c4e668f,1000000000000000000' \
  --blockTime 1 \
  > $DATA_DIR/ganache.log 2>&1 &