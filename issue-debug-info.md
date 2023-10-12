logs of processor are different
https://github.com/CoreumFoundation/coreum/actions/runs/6481841533/job/17600159168

hermes --config /app/.hermes/config.toml query packet pending --chain coreum-devnet-1 --port wasm.devcore14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sd4f0ak --channel channel-1

```
SUCCESS Summary {
    src: PendingPackets {
        unreceived_packets: [
            Collated {
                start: Sequence(
                    1,
                ),
                end: Sequence(
                    2,
                ),
            },
        ],
        unreceived_acks: [],
    },
    dst: PendingPackets {
        unreceived_packets: [
            Collated {
                start: Sequence(
                    1,
                ),
                end: Sequence(
                    3,
                ),
            },
        ],
        unreceived_acks: [],
    },
}
```

hermes --config /app/.hermes/config.toml query packet pending --chain osmosis-localnet-1 --port wasm.osmo14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sq2r9g9 --channel channel-1
```
SUCCESS Summary {
    src: PendingPackets {
        unreceived_packets: [
            Collated {
                start: Sequence(
                    1,
                ),
                end: Sequence(
                    3,
                ),
            },
        ],
        unreceived_acks: [],
    },
    dst: PendingPackets {
        unreceived_packets: [
            Collated {
                start: Sequence(
                    1,
                ),
                end: Sequence(
                    2,
                ),
            },
        ],
        unreceived_acks: [],
    },
}
```
hermes --config /app/.hermes/config.toml query packet commitments --chain coreum-devnet-1 --port wasm.devcore14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sd4f0ak --channel channel-1

```
SUCCESS PacketSeqs {
    height: Height {
        revision: 1,
        height: 2430,
    },
    seqs: [
        Collated {
            start: Sequence(
                1,
            ),
            end: Sequence(
                2,
            ),
        },
    ],
}
```

// Need to copy keys because we run from root user.
mkdir -p /root/.hermes/keys/ && cp -r app/.hermes/keys/* /root/.hermes/keys/

hermes --config /app/.hermes/config.toml tx packet-recv --dst-chain osmosis-localnet-1 --src-chain coreum-devnet-1 --src-port wasm.devcore14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sd4f0ak --src-channel channel-1
hermes --config /app/.hermes/config.toml tx packet-recv --dst-chain coreum-devnet-1 --src-chain osmosis-localnet-1 --src-port wasm.osmo14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sq2r9g9 --src-channel channel-1

// We can also query for tx send by relayer and try text search by contract name and see what is inside of tx.