# Can7

This repo contains a handful of terminal-based utilities for viewing real-time
diagnostics using the CAN interface of [MBE](https://www.mbesystems.com/) engine
control units, as found in modern
[Caterham 7](https://en.wikipedia.org/wiki/Caterham_7) sports cars.

I built these to enable me to reset the throttle position sensor on my own car,
as the manufacturer reset process requires access to proprietary hardware and a
companion free-to-download proprietary Windows application (Easimap).

<p align="center">
  <img width="650" src="https://www.oesmith.co.uk/img/caterscan-demo.svg">
</p>

I use a Raspberry Pi with a
[Waveshare 2-channel CAN HAT](https://www.waveshare.com/wiki/2-CH_CAN_HAT). You
may have success with similar hardware.

Credit must be given to John Martin of the Purplemeanie blog for his series of
articles on
[ECU diagnostics](https://purplemeanie.co.uk/index.php/2019/08/31/ecu-diagnostics-part-1-introduction/).
If you're interested in learning more about the MBE ECU in Caterhams, this is a
great resource.

## Usage

First you must configure your CAN interface to work with the MBE ECU. On
Raspbian, you can do this by adding the following to
`/etc/network/interfaces.d/can`:

```
auto can0
iface can0 inet manual
    pre-up /sbin/ip link set can0 type can bitrate 500000 triple-sampling on restart-ms 100
    up /sbin/ifconfig can0 up
    down /sbin/ifconfig can0 down
```

All commands are exepected to be executed with the ignition on, and, if querying
live engine metrics, with the engine running.

### Identifying your ECU

The `mbe-ver` utility will extract the serial number from your ECU.

To install:

```
$ go install github.com/oesmith/can7/cmd/mbe-ver@latest
```

Usage:

```
$ mbe-ver --device can0
ECU version #959bd804
```

### Running diagnostics

The `caterscan` utility displays live realtime diagnostics by reading. The
diagnostic parameters to fetch and display are configured in a YAML file
containing the ECU page and offset for each parameter, and the scaling
parameters to translate the ECU values to human-readable metrics.

I've provided an example file that works with my own car. However, the locations
of diagnostic parameters will change in other ECU models. If you need to develop
a custom config for your own car, then you can find a full catalog in the config
files shipped with the proprietary Easimap software.

To install:

```
$ go install github.com/oesmith/can7/cmd/caterscan@latest
```

Usage:

```
$ caterscan --device can0 --config params.yaml
```

## Development

As well as the diagnostics utilities mentioned above, this repository also
contains a small handful of other utilities that I used when in development.

- `bcast` - Dumps diagnostic parameters from the more primitive broadcast
  protocol used by the MBE ECU.
- `replay` - Replays a wireshark / tshark session dump on the CAN bus.
- `mbe-fake` - A fake implementation of the ECU-end of the CAN protocol.

These tools are typically used with a virtual CAN interface, which can be setup
using the following commands:

```
sudo modprobe vcan
sudo ip link add dev vcan0 type vcan
sudo ip link set up vcan0
```
