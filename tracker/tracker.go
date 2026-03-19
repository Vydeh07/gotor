package tracker

import (
	"encoding/binary"
	"fmt"
	"gotor/bencode"
	"gotor/torrent"
	"io"
	"math/rand"
	"net/http"
	"net/url"
)

type Peer struct {
	IP   string
	Port uint16
}

func GeneratePeerID() [20]byte {
	var id [20]byte
	rand.Read(id[:])
	return id
}
func urlEncodeBytes(b []byte) string {
	encoded := ""
	for _, byte_ := range b {
		encoded += fmt.Sprintf("%%%02x", byte_)
	}
	return encoded
}
func GetPeers(tf *torrent.TorrentFile, peerID [20]byte) ([]Peer, error) {
	params := url.Values{}
	params.Set("port", "6881")
	params.Set("uploaded", "0")
	params.Set("downloaded", "0")
	params.Set("left", fmt.Sprintf("%d", tf.Length))
	params.Set("compact", "1")

	trackerURL := fmt.Sprintf("%s?%s&info_hash=%s&peer_id=%s",
		tf.Announce,
		params.Encode(),
		urlEncodeBytes(tf.InfoHash[:]),
		urlEncodeBytes(peerID[:]),
	)

	resp, err := http.Get(trackerURL)
	if err != nil {
		return nil, fmt.Errorf("tracker request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	decoded, err := bencode.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tracker response: %w", err)
	}

	if failureVal, ok := decoded.DictVal["failure reason"]; ok {
		return nil, fmt.Errorf("tracker error: %s", string(failureVal.StringVal))
	}

	peersVal, ok := decoded.DictVal["peers"]
	if !ok {
		return nil, fmt.Errorf("no peers in response")
	}
	peersBin := peersVal.StringVal

	if len(peersBin)%6 != 0 {
		return nil, fmt.Errorf("invalid peers length")
	}

	var peers []Peer
	for i := 0; i < len(peersBin); i += 6 {
		ip := fmt.Sprintf("%d.%d.%d.%d",
			peersBin[i],
			peersBin[i+1],
			peersBin[i+2],
			peersBin[i+3],
		)
		port := binary.BigEndian.Uint16(peersBin[i+4 : i+6])
		peers = append(peers, Peer{IP: ip, Port: port})
	}

	return peers, nil
}

func GetAllPeers(tf *torrent.TorrentFile, peerID [20]byte) ([]Peer, error) {
	seen := make(map[string]bool)
	var allPeers []Peer

	// try all trackers — main + backups
	allTrackers := append([]string{tf.Announce}, tf.AnnounceList...)

	for _, trackerURL := range allTrackers {
		// temporarily swap announce URL
		original := tf.Announce
		tf.Announce = trackerURL
		peers, err := GetPeers(tf, peerID)
		tf.Announce = original

		if err != nil {
			fmt.Printf("Tracker %s failed: %v\n", trackerURL, err)
			continue
		}

		for _, p := range peers {
			key := fmt.Sprintf("%s:%d", p.IP, p.Port)
			if !seen[key] {
				seen[key] = true
				allPeers = append(allPeers, p)
			}
		}

		fmt.Printf("Tracker %s gave %d peers\n", trackerURL, len(peers))
	}

	if len(allPeers) == 0 {
		return nil, fmt.Errorf("no peers found from any tracker")
	}

	return allPeers, nil
}
