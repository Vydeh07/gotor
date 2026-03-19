package torrent

import (
	"crypto/sha1"
	"fmt"
	"gotor/bencode"
	"os"
)

type TorrentFile struct {
	Announce     string
	AnnounceList []string // backup trackers
	Name         string
	Length       int64
	PieceLength  int64
	Pieces       [][]byte
	InfoHash     [20]byte
}

func Parse(filename string) (*TorrentFile, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	val, err := bencode.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bencode: %w", err)

	}

	if val.Type != "dict" {
		return nil, fmt.Errorf("expected dict at top level")
	}
	tf := &TorrentFile{}

	announce, ok := val.DictVal["announce"]
	if !ok {
		return nil, fmt.Errorf("missing announce key")
	}
	tf.Announce = string(announce.StringVal)

	// ADD THIS BLOCK RIGHT HERE
	if announceList, ok := val.DictVal["announce-list"]; ok {
		for _, tier := range announceList.ListVal {
			for _, trackerVal := range tier.ListVal {
				url := string(trackerVal.StringVal)
				if url != tf.Announce {
					tf.AnnounceList = append(tf.AnnounceList, url)
				}
			}
		}
	}

	info, ok := val.DictVal["info"]
	if !ok {
		return nil, fmt.Errorf("missing info key")
	}
	tf.InfoHash = sha1.Sum(info.RawBytes)

	name, ok := info.DictVal["name"]
	if !ok {
		return nil, fmt.Errorf("missing info.name")
	}
	tf.Name = string(name.StringVal)

	length, ok := info.DictVal["length"]
	if !ok {
		return nil, fmt.Errorf("missing info.length")
	}
	tf.Length = length.IntVal

	pieceLength, ok := info.DictVal["piece length"]
	if !ok {
		return nil, fmt.Errorf("missing info.piece length")
	}
	tf.PieceLength = pieceLength.IntVal
	pieces, ok := info.DictVal["pieces"]
	if !ok {
		return nil, fmt.Errorf("missing info.pieces")
	}
	raw := pieces.StringVal
	if len(raw)%20 != 0 {
		return nil, fmt.Errorf("pieces length is not a multiple of 20")
	}
	for i := 0; i < len(raw); i += 20 {
		tf.Pieces = append(tf.Pieces, raw[i:i+20])
	}

	return tf, nil
}
