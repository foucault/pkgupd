package main

import "github.com/jessevdk/go-flags"

// Options is a struct holding all the command line arguments
type Options struct {
	// Enable automatic sync
	EnableSync bool `short:"s" long:"enable-sync" default:"false" description:"Enable automatic sync of dbs"`
	// Enable AUR sync
	EnableAUR bool `short:"a" long:"enable-aur" default:"false" description:"Check foreign packages for updates in AUR"`
	// Interval between database sync (seconds)
	SyncInterval int `long:"sync-interval" default:"1800" description:"Interval for database sync in seconds"`
	// Interval between AUR sync (second)
	AURInterval int `long:"aur-interval" default:"1800" description:"Interval for AUR checks"`
	// Path of the pacman.conf
	PacmanConf flags.Filename `long:"pacman-conf" default:"/etc/pacman.conf" description:"Pacman configuration file"`
	// Interval between regular repo update
	PollInterval int `long:"poll-interval" default:"600" description:"Interval for repo updates"`
	// Verbose message, use twice for more
	Verbose []bool `short:"v" long:"verbose" description:"Enable verbose logging"`
	// Path of the sandbox database
	DBRoot flags.Filename `short:"d" long:"db-root" default:"/tmp/pkgupd-sandbox" description:"Local/sync database root directory"`
	// Type of the listening socket (tcp or unix)
	ListenType string `short:"l" long:"listen-type" default:"tcp" description:"Server listening protocol, 'tcp' or 'unix'"`
	// Address of the listening socket for tcp or socket path for unix
	ListenAddr string `short:"r" long:"listen-addr" default:":7356" description:"Address (addr:port) or socket of the server"`
	// Enable automatic updates when the pacman database changes
	NotifyFS bool `short:"m" long:"monitor-changes" default:"false" description:"Monitor pacman database for changes"`
}
