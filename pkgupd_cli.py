#!/usr/bin/python
"""
pkgupd_cli.py is a simple command line client for pkgupd. It reads data
from the server and prints the results either as a list of packages or as
a number or updatable packages.
"""

import socket
import json
import sys
import argparse

DEFAULT_SERVICES = ["repo", "aur"]

BASE_ESC = "\033[%dm"
ESC = {
    "reset":   BASE_ESC % 0,
    "bold":    BASE_ESC % 1,
    "black":   BASE_ESC % 30,
    "red":     BASE_ESC % 31,
    "green":   BASE_ESC % 32,
    "yellow":  BASE_ESC % 33,
    "blue":    BASE_ESC % 34,
    "magenta": BASE_ESC % 35,
    "cyan":    BASE_ESC % 36,
    "white":   BASE_ESC % 37
}


def _log(what, args, verbosity, alt=None, stream=sys.stderr):
    """
    Print to stream if args.verbose >= verbosity

    Args:
        what: The line to print
        args: The command line arguments
        verbosity: Minimum verbosity to display the message
        alt: What to print if verbosity check fails (default: None)
        stream: Output stream (default: sys.stderr)
    """
    if args.verbose >= verbosity:
        print(what, file=stream)
    else:
        if alt is not None:
            print(alt, file=stream)

def logstd(what, args, verbosity, alt=None):
    """
    Print to stdout if args.verbose >= verbosity

    Args:
        what: The line to print
        args: The command line arguments
        verbosity: Minimum verbosity to display the message
        alt: What to print if verbosity check fails (default: None)
    """
    _log(what, args, verbosity, alt, sys.stdout)

def logerr(what, args, verbosity, alt=None):
    """
    Print to stderr if args.verbose >= verbosity

    Args:
        what: The line to print
        args: The command line arguments
        verbosity: Minimum verbosity to display the message
        alt: What to print if verbosity check fails (default: None)
    """
    _log(what, args, verbosity, alt, sys.stderr)

def read_data(sock, srv, args):
    """
    Read and return json data from socket. This function only reads up
    to the first "\n" encountered and the rest of the response is discarded

    Args:
      rsock: The socket to read from
      rsrv: The service for which the request is made
      rargs: Dommand line arguments

    Returns:
      A dict containing the json results from the server
    """
    data = {'RequestType':srv}
    sock.send(bytes(json.dumps(data)+"\n", "UTF-8"))
    res = ""
    buf = ""
    if args.verbose > 1:
        print("Reading data from: %s"%(sock.getpeername(),), file=sys.stderr)
    while True:
        buf = sock.recv(1024).decode("UTF-8")
        if '\n' in buf:
            usable, *_ = buf.partition("\n")
            res += usable
            break
        else:
            res += buf
    rret = json.loads(res)
    return rret


def update_server(sock, args):
    """
    Forces a server sync update

    Args:
      sock: The socket to connect to
      args: Command line arguments
    """

    data = {'RequestType': "sync"}
    sock.send(bytes(json.dumps(data)+"\n", "UTF-8"))

    ret = read_data(sock, "sync", args)
    if ret["ResponseType"] == "error":
        logerr("Server returned error for service %s" % srv, args, 2)
        logerr(ret["Data"], args, 2)
    elif ret["ResponseType"] == "ok":
        logstd("OK", args, 1)
    else:
        logerr("Unknown response type", args, 2)


def process_data_normal(sock, srv, args):
    """
    Reads data from server found at rsock for service rsrv and prints
    a formatted list of the results. Depending on the verbosity level
    of rargs the output can be just a list of names or a more detailed
    list that includes versions as well

    Args:
      sock: The socket to read from
      srv: The service for which the request is made
      args: Command line arguments
    """
    verbose_color = "[%s%%s%s] %s%%s%s %s%%s%s -> %s%%s%s"%\
            (ESC["blue"], ESC["reset"], ESC["bold"], ESC["reset"],\
                ESC["yellow"], ESC["reset"], ESC["green"], ESC["reset"])
    verbose_simple = "[%s] %s %s -> %s"
    normal_color = "%s%%s%s"%(ESC["bold"], ESC["reset"])
    normal_simple = "%s"
    max_srv_len = len(args.max_srv_len)

    if args.verbose:
        if args.color and sys.stdout.isatty():
            lformat = verbose_color
        else:
            lformat = verbose_simple
    else:
        if args.color and sys.stdout.isatty():
            lformat = normal_color
        else:
            lformat = normal_simple

    logerr("Getting data for service %s"%srv, args, 2)

    ret = read_data(sock, srv, args)
    if ret["Data"] is None:
        logerr("No updates for service %s"%srv, args, 2)
    elif ret["ResponseType"] == "error":
        logerr("Server returned error for service %s"%srv, args, 2)
        logerr(ret["Data"], args, 2)
    else:
        if len(ret["Data"]) > 0:
            for item in ret["Data"]:
                if args.verbose:
                    logstd(lformat%(srv.upper().ljust(max_srv_len, " "),\
                            item["Name"], item["LocalVersion"],\
                                item["RemoteVersion"]), args, 1)
                else:
                    logstd(lformat%item["Name"], args, 0)

def process_data_numeric(sock, srv, args):
    """
    Reads data from server found at rsock for service rsrv and returns
    a number of the resulting packages.

    Args:
      rsock: The socket to read from
      rsrv: The service for which the request is made
      rargs: Command line arguments

    Returns:
      A count of the resulting packages
    """
    ret = read_data(sock, srv, args)
    if ret["Data"] is None:
        return 0
    elif ret["ResponseType"] == "error":
        return "NA"
    else:
        return len(ret["Data"])

def init_parser():
    """
    Initializes the command line parser

    Returns:
      The argpars.ArgumentParser() for the program
    """
    verbose_help = "Print a more detailed report"
    sync_help = "Force update of the sync state and exit. "+\
            "Other arguments will be ignored"
    numeric_help = "Print the number of updates per service, "+\
            "missing services will be replaced by NA"
    service_help = "List of services to query, missing services will be ignored"
    type_help = "Type of connection \"tcp\" or \"unix\""
    port_help = "Port or socket for connection, default 7356 for tcp"
    sep_help = "Separator for numeric data, default is space"
    color_help = "Use color if outputing to terminal for non-numeric mode"
    parser = argparse.ArgumentParser()
    parser.add_argument("services", metavar="SRV", type=str, nargs="*",\
            help=service_help)
    parser.add_argument("--verbose", "-v", dest="verbose", \
            action="count", default=0, help=verbose_help)
    parser.add_argument("--force-sync", "-f", dest="force_sync", \
            action="store_true", help=sync_help)
    parser.add_argument("--color", "-c", dest="color", \
            action="store_true", help=color_help)
    parser.add_argument("--numeric", "-n", dest="numeric", \
            action="store_true", help=numeric_help)
    parser.add_argument("--separator", "-s", dest="separator",\
            action="store", default=" ", help=sep_help)
    parser.add_argument("--type", "-t", dest="type",\
            action="store", default="tcp", help=type_help)
    parser.add_argument("--port", "-p", dest="port",\
            action="store", default="7356", help=port_help)
    return parser

def main():
    """ The main function """
    arg_parser = init_parser()
    args = arg_parser.parse_args()
    if args.verbose and args.numeric:
        print("ERROR: Can't use both verbose and numeric modes",\
                file=sys.stderr)
        sys.exit(1)
    if len(args.services) == 0:
        services = DEFAULT_SERVICES
    else:
        services = args.services

    sock = None

    if args.type == "tcp":
        try:
            sock = socket.create_connection(("localhost", int(args.port)))
        except OSError as exc:
            print("Cannot open connection to server; bailing out %s"%exc,\
                    file=sys.stderr)
            sys.exit(1)
    elif args.type == "unix":
        try:
            sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            sock.connect(args.port)
        except OSError as exc:
            print("Cannot open connection to server; bailing out %s"%exc,\
                    file=sys.stderr)
            sys.exit(1)
    else:
        print("ERROR: Unknown connection type, must be \"tcp\" or \"unix\"",\
                file=sys.stderr)
        sys.exit(1)

    # add some "meta options"
    args.max_srv_len = max(services, key=len)

    if args.force_sync:
        update_server(sock, args)
        return

    if args.numeric:
        ret = []
        for srv in services:
            ret.append(process_data_numeric(sock, srv, args))
        print(args.separator.join([str(x) for x in ret]))
    else:
        for srv in services:
            process_data_normal(sock, srv, args)

    sock.close()

if __name__ == "__main__":
    main()

