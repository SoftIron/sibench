package main

import "encoding/json"
import "github.com/docopt/docopt-go"
import "fmt"
import "io/ioutil"
import "logger"
import "math"
import "os"
import "regexp"
import "strings"
import "strconv"
import "time"


/* Struct type into which DocOpt can put our command line options. */
type Arguments struct {
    // Command selection bools
    Server bool
    S3 bool
    Rados bool
    Cephfs bool
    Run bool
    Verbose bool

    // Common options
    Port int
    MountsDir string
    Size string
    Objects int
    Servers string
    RunTime int
    RampUp int
    RampDown int
    Bandwidth string
    JsonOutput string
    Targets []string

    // S3 options
    S3AccessKey string
    S3SecretKey string
    S3Bucket string
    S3Port int

    // Rados and/or CephFS options
    CephPool string
    CephUser string
    CephKey  string
    CephDir  string

    // Synthesized options
    Bucket string
    BandwidthInBytes uint64
    SizeInBytes uint64
}


/* Return a usage string for DocOpt argument parsing. */
func usage() string {
    return `SoftIron Benchmark Tool.
Usage:
  sibench server     [-v] [-p PORT] [-m DIR]
  sibench s3 run     [-v] [-p PORT] [-s SIZE] [-o COUNT] [-r TIME] [-u TIME] [-d TIME] [-b BW] [-j FILE] [--servers SERVERS] <targets> ...
                     [--s3-port PORT] [--s3-bucket BUCKET] (--s3-access-key KEY) (--s3-secret-key KEY)
  sibench rados run  [-v] [-p PORT] [-s SIZE] [-o COUNT] [-r TIME] [-u TIME] [-d TIME] [-b BW] [-j FILE] [--servers SERVERS] <targets> ...
                     [--ceph-pool POOL] [--ceph-user USER] (--ceph-key KEY)
  sibench cephfs run [-v] [-p PORT] [-s SIZE] [-o COUNT] [-r TIME] [-u TIME] [-d TIME] [-b BW] [-j FILE] [-m DIR] [--servers SERVERS] <targets> ...
                     [--ceph-dir DIR] [--ceph-user USER] (--ceph-key KEY)
  sibench -h | --help

Options:
  -h, --help                   Show full usage
  -v, --verbose                Turn on debug output.
  -p PORT, --port PORT         The port on which sibench communicates.                          [default: 5150]
  -m DIR, --mounts-dir DIR     The directory in which we should create any filesystem mounts.   [default: /tmp/sibench_mnt]
  -s SIZE, --size SIZE         Object size to test, in units of K or M.                         [default: 1M]
  -o COUNT, --objects COUNT    The number of objects to use as our working set.                 [default: 1000]
  -r TIME, --run-time TIME     Seconds spent on each phase of the benchmark.                    [default: 30]
  -u TIME, --ramp-up TIME      Seconds at the start of each phase where we don't record data.   [default: 5]
  -d TIME, --ramp-down TIME    Seconds at the end of each phase where we don't record data.     [default: 2]
  -j FILE, --json-output FILE  The file to which we write our json results.                     [default: sibench.json]
  -b BW, --bandwidth BW        Benchmark at a fixed bandwidth, in units of K, M or G bits/s..   [default: 0]
  --servers SERVERS            A comma-separated list of sibench servers to connect to.         [default: localhost]
  --s3-port PORT               The port on which to connect to S3.                              [default: 7480]
  --s3-bucket BUCKET           The name of the bucket we wish to use for S3 operations.         [default: sibench]
  --s3-access-key KEY          S3 access key.
  --s3-secret-key KEY          S3 secret key.
  --ceph-pool POOL             The pool we use for benchmarking.                                [default: sibench]
  --ceph-user USER             The ceph username we use.                                        [default: admin]
  --ceph-key KEY               The secret key belonging to the ceph user.
  --ceph-dir DIR               The CephFS directory which we should use for a benchmark.        [default: sibench]
`
}


/* Helper function to dump an object to string in a nice way */
func prettyPrint(i interface{}) string {
    j, err := json.MarshalIndent(i, "", "  ")
    if err != nil {
        return fmt.Sprintf("Error printing %v: %v", i, err)
    }

    return string(j)
}


/* 
 * Helper to simplify our error handling.  
 * If err is not nil, then we print an error message and die (with a non-zero exit code).
 */
func dieOnError(err error, format string, a ...interface{}) {
    if err != nil {
        fmt.Printf(format, a)
        fmt.Printf(": %v\n", err)
        os.Exit(-1)
    }
}


/* 
 * Convert a string with optional units into an uint, expanding the units.
 * The units accepted are [None] or K, M, G in either upper or lower case.
 *
 * Eg:  1->1, 1k->1024, 1m->1048576 etc.
 */
func expandUnits(val string) (uint64, error) {
    // A regex for converting numbers with optional units (in K, M or G) into long form.
    re := regexp.MustCompile(`([0-9]+)([kKmMgG]?)$`)

    // Turn the size (in K, M or G) into bytes...
    groups := re.FindStringSubmatch(val)
    if groups == nil {
        return 0, fmt.Errorf("Bad size specifier: %v", val)
    }

    ival, _ := strconv.Atoi(groups[1])
    uval := uint64(ival)

    switch strings.ToLower(groups[2]) {
        case "k": uval *= 1024
        case "m": uval *= 1024 * 1024
        case "g": uval *= 1024 * 1024 * 1024
    }

    return uval, nil
}


/* 
 * Do any argument checking that can not be done inherently by DocOpt (such as 
 * ensuring a port number is < 65535, or that a string has a particular form.
 */
func validateArguments(args *Arguments) error {
    if (args.Port < 0) || ( args.Port > int(math.MaxUint16)) {
        return fmt.Errorf("Port not in range: %v", args.Port)
    }

    if (args.S3Port < 0) || ( args.S3Port > int(math.MaxUint16)) {
        return fmt.Errorf("S3 Port not in range: %v", args.S3Port)
    }

    var err error
    args.SizeInBytes, err = expandUnits(args.Size)
    if err != nil {
        return err
    }

    args.BandwidthInBytes, err = expandUnits(args.Bandwidth)
    if err != nil {
        return err
    }

    args.BandwidthInBytes /= 8

    return nil
}


/*
 * Build our Config.
 *
 * Currently this uses just our command line arguments, but it will probably load a json file later on.
 */
func buildConfig(args *Arguments) error {
    config.ListenPort = uint16(args.Port)
    config.MountsDir = args.MountsDir
    return nil
}


func main() {
    // Error should never happen outside of development, since docopt is complaining that our usage string has bad syntax.
    opts, err := docopt.ParseDoc(usage())
    dieOnError(err, "Error parsing arguments")

    // Error should never happen outside of development, since docopt is complaining that our type bindings are wrong.
    var args Arguments
    err = opts.Bind(&args)
    dieOnError(err, "Failure binding argsig")

    // This can error on bad user input.
    err = validateArguments(&args)
    dieOnError(err, "Failure validating arguments")

    // Build our config.  In the future, this may load json etc...
    err = buildConfig(&args)
    dieOnError(err, "Failure building config")

    if args.Verbose {
        fmt.Printf("%v\n", prettyPrint(args))
        logger.SetLevel(logger.Debug)
    }

    if args.Server {
        startServer(&args)
    }

    if args.Run {
        startRun(&args)
    }
}


/* Start a server, listening on a TCP port */
func startServer(args *Arguments) {

    err := StartForeman()
    dieOnError(err, "Failure creating server")
}


/* Create a job and execute it on some set of servers. */
func startRun(args *Arguments) {
    var j Job

    j.servers = strings.Split(args.Servers, ",")
    j.serverPort = uint16(args.Port)
    j.runTime = uint64(args.RunTime)
    j.rampUp = uint64(args.RampUp)
    j.rampDown = uint64(args.RampDown)

    j.order.JobId = 1
    j.order.ObjectSize = args.SizeInBytes
    j.order.Seed = uint64(time.Now().Unix())
    j.order.GeneratorType = "prng"
    j.order.RangeStart = 0
    j.order.RangeEnd = uint64(args.Objects)
    j.order.Targets = args.Targets
    j.order.Bandwidth = args.BandwidthInBytes

    if args.S3 {
        j.order.ConnectionType = "s3"
        j.order.Bucket = args.S3Bucket
        j.order.Credentials = map[string]string { "access_key": args.S3AccessKey, "secret_key": args.S3SecretKey }
        j.order.Port = uint16(args.S3Port)
    } else if args.Rados {
        j.order.ConnectionType = "rados"
        j.order.Bucket = args.CephPool
        j.order.Credentials = map[string]string { "username": args.CephUser, "key": args.CephKey }
    } else {
        j.order.ConnectionType = "cephfs"
        j.order.Bucket = args.CephDir
        j.order.Credentials = map[string]string { "username": args.CephUser, "key": args.CephKey }
    }

    j.setArguments(args)
    m := NewManager()

    err := m.Run(&j)
    if err != nil {
        fmt.Printf("Error running job: %v\n", err)
        j.addError(err)
    }

    jsonReport, err := json.MarshalIndent(j.report, "", "  ")
    dieOnError(err, "Unable to encode results as json")

    if args.JsonOutput != "" {
        err = ioutil.WriteFile(args.JsonOutput, jsonReport, 0644)
        dieOnError(err, "Unable to write json report to file: %v", args.JsonOutput)
    }

    logger.Infof("Done\n")
}

