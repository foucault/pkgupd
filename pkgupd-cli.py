#!/usr/bin/python

import socket
import json
import sys
import argparse

DEFAULT_SERVICES = ["repo","aur"]

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

def read_data(sock, srv):
    data = {'RequestType':srv}
    sock.send(bytes(json.dumps(data)+"\n", "UTF-8"))
    res = ""
    buf = ""
    while True:
        buf = sock.recv(1024).decode("UTF-8")
        if '\n' in buf:
            usable, delim, rej = buf.partition("\n")
            res += usable
            break
        else:
            res += buf
    ret = json.loads(res)
    return ret

def process_data_normal(sock, srv, args):
    verbose_color = "[%s%%s%s] %s%%s%s %s%%s%s -> %s%%s%s"%\
            (ESC["blue"],ESC["reset"],ESC["bold"],ESC["reset"],\
                ESC["yellow"],ESC["reset"],ESC["green"],ESC["reset"])
    verbose_simple = "[%s] %s %s -> %s"
    normal_color = "%s%%s%s"%(ESC["bold"],ESC["reset"])
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

    ret = read_data(sock, srv)
    if ret["Data"] is None:
        if args.verbose:
            print("No updates for service %s"%srv)
    elif ret["ResponseType"] == "error":
        if args.verbose:
            print("Server returned error for service %s"%srv, file=sys.stderr)
            print(ret["Data"], file=sys.stderr)
    else:
        if len(ret["Data"]) > 0:
            for item in ret["Data"]:
                if item["Foreign"]:
                    if args.verbose:
                        print(lformat%(srv.upper().ljust(max_srv_len," "),\
                                item["Name"], item["LocalVersion"],\
                                    item["RemoteVersion"]))
                    else:
                        print(lformat%item["Name"])
                else:
                    if args.verbose:
                        print(lformat%(srv.upper().ljust(max_srv_len," "),\
                                item["Name"], item["LocalVersion"],\
                                    item["RemoteVersion"]))
                    else:
                        print(lformat%item["Name"])

def process_data_numeric(sock, srv, args):
    ret = read_data(sock, srv)
    if ret["Data"] is None:
        return 0
    elif ret["ResponseType"] == "error":
        return "NA"
    else:
        return len(ret["Data"])

def init_parser():
    verbose_help = "Print a more detailed report"
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
            action="store_true", help=verbose_help)
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

if __name__ == "__main__":
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
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.connect(('127.0.0.1',int(args.port)))
        except Exception as exc:
            print("Cannot open connection to server; bailing out %s"%exc,\
                    file=sys.stderr)
            sys.exit(1)
    elif args.type == "unix":
        try:
            sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            sock.connect(args.port)
        except Exception as exc:
            print("Cannot open connection to server; bailing out %s"%exc,\
                    file=sys.stderr)
            sys.exit(1)
    else:
        print("ERROR: Unknown connection type, must be \"tcp\" or \"unix\"",\
                file=sys.stderr)
        sys.exit(1)

    # add some "meta options"
    args.max_srv_len = max(services, key=len)

    if args.numeric:
        ret = []
        for srv in services:
            ret.append(process_data_numeric(sock, srv, args))
        print(args.separator.join([str(x) for x in ret]))
    else:
        for srv in services:
            process_data_normal(sock, srv, args)

    sock.close()

