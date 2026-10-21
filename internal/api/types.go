// Package api defines the small JSON contract shared by the CLI and launcher.
// HTTP is framing over a local Unix socket, not a network API.
package api

import "time"

type Error struct {
	Error string `json:"error"`
}

type CreateOutpostRequest struct {
	Name      string `json:"name"`
	ImageId   string `json:"image_id"`
	Vcpus     int    `json:"vcpus"`
	MemoryMib int    `json:"memory_mib"`
	DiskGib   int    `json:"disk_gib"`
}

type Outpost struct {
	Id           string    `json:"id"`
	Name         string    `json:"name"`
	ImageId      string    `json:"image_id"`
	Vcpus        int       `json:"vcpus"`
	MemoryMib    int       `json:"memory_mib"`
	DiskGib      int       `json:"disk_gib"`
	DesiredState string    `json:"desired_state"`
	Status       string    `json:"status"`
	GuestIp      string    `json:"guest_ip"`
	Failure      string    `json:"failure"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
