package main

import (
	"fmt"
	"gotor/downloader"
	"gotor/torrent"
	"gotor/tracker"
)

func main() {
	tf, err := torrent.Parse("test.torrent")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Name:       ", tf.Name)
	fmt.Println("Size:       ", tf.Length, "bytes")
	fmt.Println("Pieces:     ", len(tf.Pieces))

	peerID := tracker.GeneratePeerID()
	peers, err := tracker.GetAllPeers(tf, peerID)
	if err != nil {
		fmt.Println("Tracker error:", err)
		return
	}

	fmt.Printf("Got %d peers\n", len(peers))

	err = downloader.Download(tf, peers, peerID, tf.Name)
	if err != nil {
		fmt.Println("Download error:", err)
		return
	}
}
