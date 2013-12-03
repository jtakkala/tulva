Tulva
=====

Tulva is BitTorrent client written entirely in Go as a project for Hacker School fall 2013. It's currently a work in progress. We aim for correctness and protocol completeness: designing an optimal download strategy, implementing both upload and download, multiple tracker support, and the resumption of interrupted downloads. The authors are Jari Takkala and Brian Dignan. The name Tulva comes from the Finnish word for flood.

## Status

### Completed
- Parse torrent file
- Verifies partially downloaded files on restart
- Connects to tracker and retrieves a list of peers
- Periodically reconnects to tracker every X interval
- Connects to peers returned by the tracker
- Initalizes a server process and accepts connections from peers
- Completes peer handshake
- Controller logic and tests 90% complete

### To-do
- Handle multiple trackers and backup trackers
- Initialize local bitfield
- Send bitfield to peers
- Peer wire protocol
- Verify received pieces and write to disk
- Read pieces from disk and write to peer
- Track torrent and peer statistics and report back to tracker
- Distributed Hash Table (DHT) support
- Web interface for managing torrents
- Support multiple simultaneous torrents

## License
Tulva is licensed under the 2-clause BSD license. See LICENSE for more information.
