package main

import "github.com/jessevdk/go-flags"

type Options struct {
	EnableSync   bool           `short:"s" long:"enable-sync" default:"false" description:"Enable automatic sync of dbs"`
	EnableAUR    bool           `short:"a" long:"enable-aur" default:"false" description:"Check foreign packages for updates in AUR"`
	SyncInterval int            `long:"sync-interval" default:"1800" description:"Interval for database sync in seconds"`
	AURInterval  int            `long:"aur-interval" default:"1800" description:"Interval for AUR checks"`
	PacmanConf   flags.Filename `long:"pacman-conf" default:"/etc/pacman.conf" description:"Pacman configuration file"`
	PollInterval int            `long:"poll-interval" default:"600" description:"Interval for repo updates"`
	Verbose      []bool         `short:"v" long:"verbose" description:"Enable verbose logging"`
	DBRoot       flags.Filename `short:"d" long:"db-root" default:"/tmp/pkgupd-sandbox" description:"Local/sync database root directory"`
	ListenType   string         `short:"l" long:"listen-type" default:"tcp" description:"Server listening protocol, 'tcp' or 'unix'"`
	ListenAddr   string         `short:"r" long:"listen-addr" default:":7356" description:"Address (addr:port) or socket of the server"`
}
