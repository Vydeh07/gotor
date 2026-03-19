package downloader

import (
	"crypto/sha1"
	"fmt"
	"gotor/peer"
	"gotor/torrent"
	"gotor/tracker"
	"os"
	"time"
)

const BlockSize = 16384
const MaxPipeline = 5

type pieceWork struct {
	index  int
	hash   [20]byte
	length int
}

type pieceResult struct {
	index int
	data  []byte
}

func pieceLength(tf *torrent.TorrentFile, index int) int {
	start := index * int(tf.PieceLength)
	end := start + int(tf.PieceLength)
	if end > int(tf.Length) {
		end = int(tf.Length)
	}
	return end - start
}

func downloadPiece(conn *peer.Connection, work pieceWork) ([]byte, error) {
	buf := make([]byte, work.length)
	downloaded := 0
	requested := 0
	pending := 0

	conn.Conn.SetDeadline(time.Now().Add(30 * time.Second))

	for downloaded < work.length {
		for pending < MaxPipeline && requested < work.length {
			blockSize := BlockSize
			if work.length-requested < blockSize {
				blockSize = work.length - requested
			}

			payload := make([]byte, 12)
			payload[0] = byte(work.index >> 24)
			payload[1] = byte(work.index >> 16)
			payload[2] = byte(work.index >> 8)
			payload[3] = byte(work.index)
			payload[4] = byte(requested >> 24)
			payload[5] = byte(requested >> 16)
			payload[6] = byte(requested >> 8)
			payload[7] = byte(requested)
			payload[8] = byte(blockSize >> 24)
			payload[9] = byte(blockSize >> 16)
			payload[10] = byte(blockSize >> 8)
			payload[11] = byte(blockSize)

			err := conn.SendMessage(peer.Message{ID: peer.MsgRequest, Payload: payload})
			if err != nil {
				return nil, fmt.Errorf("failed to send request: %w", err)
			}
			requested += blockSize
			pending++
		}

		msg, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("failed to read message: %w", err)
		}

		switch msg.ID {
		case peer.MsgChoke:
			return nil, fmt.Errorf("peer choked us")

		case peer.MsgUnchoke:

		case peer.MsgHave:

		case peer.MsgPiece:
			if len(msg.Payload) < 8 {
				return nil, fmt.Errorf("piece message too short")
			}
			offset := int(msg.Payload[4])<<24 | int(msg.Payload[5])<<16 |
				int(msg.Payload[6])<<8 | int(msg.Payload[7])
			data := msg.Payload[8:]
			copy(buf[offset:], data)
			downloaded += len(data)
			pending--
		}
	}

	return buf, nil
}

func verifyPiece(data []byte, expected [20]byte) bool {
	hash := sha1.Sum(data)
	return hash == expected
}

func startWorker(p tracker.Peer, tf *torrent.TorrentFile, peerID [20]byte,
	workQueue chan pieceWork, results chan pieceResult) {

	// try to connect up to 3 times
	var conn *peer.Connection
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		conn, err = peer.Handshake(p.IP, p.Port, tf.InfoHash, peerID)
		if err == nil {
			break
		}
		fmt.Printf("Connection attempt %d failed for %s:%d: %v\n", attempt+1, p.IP, p.Port, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		fmt.Printf("Giving up on %s:%d\n", p.IP, p.Port)
		return
	}
	defer conn.Conn.Close()

	// send interested
	err = conn.SendMessage(peer.Message{ID: peer.MsgInterested})
	if err != nil {
		fmt.Println("Failed to send interested:", err)
		return
	}

	// wait for unchoke — but handle ALL message types while waiting
	// peers often send bitfield, have, etc before unchoke
	unchoked := false
	conn.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	for !unchoked {
		msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Printf("Error waiting for unchoke from %s:%d: %v\n", p.IP, p.Port, err)
			return
		}
		switch msg.ID {
		case peer.MsgUnchoke:
			unchoked = true
		case peer.MsgBitfield:
			conn.Bitfield = msg.Payload
		case peer.MsgHave:
			// ignore for now
		case peer.MsgChoke:
			fmt.Printf("Peer %s:%d choked us immediately\n", p.IP, p.Port)
			return
		default:
			// ignore unknown messages
		}
	}

	fmt.Printf("Unchoked by %s:%d — starting download\n", p.IP, p.Port)

	for work := range workQueue {
		// reset deadline for each piece
		conn.Conn.SetDeadline(time.Now().Add(30 * time.Second))

		data, err := downloadPiece(conn, work)
		if err != nil {
			fmt.Printf("Failed piece %d from %s:%d: %v\n", work.index, p.IP, p.Port, err)
			workQueue <- work // put it back
			return
		}

		if !verifyPiece(data, work.hash) {
			fmt.Printf("Piece %d failed verification\n", work.index)
			workQueue <- work
			continue
		}

		results <- pieceResult{index: work.index, data: data}
	}
}

func Download(tf *torrent.TorrentFile, peers []tracker.Peer, peerID [20]byte, outputPath string) error {
	// create the output file upfront
	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// pre-allocate the file size on disk
	err = file.Truncate(tf.Length)
	if err != nil {
		return fmt.Errorf("failed to allocate file: %w", err)
	}

	workQueue := make(chan pieceWork, len(tf.Pieces))
	results := make(chan pieceResult)

	for i, hash := range tf.Pieces {
		var h [20]byte
		copy(h[:], hash)
		workQueue <- pieceWork{
			index:  i,
			hash:   h,
			length: pieceLength(tf, i),
		}
	}

	for _, p := range peers {
		go startWorker(p, tf, peerID, workQueue, results)
	}

	done := 0
	total := len(tf.Pieces)

	for done < total {
		result := <-results

		// write this piece directly to the correct position in the file
		offset := int64(result.index) * tf.PieceLength
		_, err := file.WriteAt(result.data, offset)
		if err != nil {
			return fmt.Errorf("failed to write piece %d: %w", result.index, err)
		}

		done++
		fmt.Printf("\rProgress: %d/%d pieces (%.1f%%)", done, total,
			float64(done)/float64(total)*100)
	}
	close(workQueue)

	fmt.Println("\nDownload complete! Saved to", outputPath)
	return nil
}
