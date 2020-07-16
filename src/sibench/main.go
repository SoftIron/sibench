package main

import "encoding/json"
import "github.com/docopt/docopt-go"
import "fmt"
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
    Run bool

    // Common options
    Port int
    Size string
    Objects int
    Servers string
    RunTime int
    RampUp int
    RampDown int
    Targets []string

    // S3 options
    S3AccessKey string
    S3SecretKey string
    S3Bucket string
    S3Port int

    // Rados options
    CephPool string
    CephUser string
    CephKey  string

    // Synthesized options
    Bucket string
    SizeInBytes uint64
}


/* Return a usage string for DocOpt argument parsing. */
func usage() string {
    return `SoftIron Benchmark Tool.
Usage:
  sibench server    [-p PORT]
  sibench s3 run    [-p PORT] [-s SIZE] [-o COUNT] [-r TIME] [-u TIME] [-d TIME] [--servers SERVERS] <targets> ...
                    [--s3-port PORT] [--s3-bucket BUCKET] (--s3-access-key KEY) (--s3-secret-key KEY)
  sibench rados run [-p PORT] [-s SIZE] [-o COUNT] [-r TIME] [-u TIME] [-d TIME] [--servers SERVERS] <targets> ...
                    [--ceph-pool POOL] [--ceph-user USER] (--ceph-key KEY)
Options:
  -p PORT, --port PORT         The port on which sibench communicates.  [default: 5150]
  -s SIZE, --size SIZE         Object size to test, in units of K or M.   [default: 1M]
  -o COUNT, --objects COUNT    The number of objects to use as our working set.  [default: 1000]
  -r TIME, --run-time TIME     The time spent on each phase of the benchmark.  [default: 30]
  -u TIME, --ramp-up TIME      The extra time we run at the start of each phase where we don't collect stats.  [default: 5]
  -d TIME, --ramp-down TIME    The extra time we run at the end of each phase where we don't collect stats.  [default: 2]
  --servers SERVERS            A comma-separated list of sibench servers to connect to.  [default: localhost]
  --s3-port PORT               The port on which to connect to S3.  [default: 7480]
  --s3-bucket BUCKET           The name of the bucket we wish to use for S3 operations.  [default: sibench]
  --s3-access-key KEY          S3 access key
  --s3-secret-key KEY          S3 secret key
  --ceph-pool POOL             The pool we use for benchmarking.  [default: sibench]
  --ceph-user USER             The ceph username we use.  [default: admin]
  --ceph-key KEY               The secret key belonging to the ceph user
`
}


/* Helper function do show the result of docopt parsing, for tracking down usage syntax issues */
func dumpOpts(opts *docopt.Opts) {
    j, err := json.MarshalIndent(opts, "", "  ")
    if err != nil {
        fmt.Println("error:", err)
    }

    fmt.Print(string(j))
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

    // Turn the size (in K or M) into bytes...

    re := regexp.MustCompile(`([1-9][0-9]*)([kKmM])`)
    groups := re.FindStringSubmatch(args.Size)
    if groups == nil {
        return fmt.Errorf("Bad size specifier: %v", args.Size)
    }

    val, _ := strconv.Atoi(groups[1])
    args.SizeInBytes = uint64(val) * 1024
    if strings.EqualFold(groups[2], "m") {
        args.SizeInBytes *= 1024
    }

    return nil
}


func main() {
    // Error should never happen outside of development, since docopt is complaining that our usage string has bad syntax.
    opts, err := docopt.ParseDoc(usage())
    dieOnError(err, "Error parsing arguments")

    dumpOpts(&opts)

    // Error should never happen outside of development, since docopt is complaining that our type bindings are wrong.
    var args Arguments
    err = opts.Bind(&args)
    dieOnError(err, "Failure binding argsig")

    // This can error on bad user input.
    err = validateArguments(&args)
    dieOnError(err, "Failure validating arguments")

    fmt.Printf("%+v\n", args)

    if args.Server {
        startServer(&args)
    }

    if args.Run {
        startRun(&args)
    }
}


/* Start a server, listening on a TCP port */
func startServer(args *Arguments) {
    err := StartForeman(uint16(args.Port))
    dieOnError(err, "Failure creating server")
}


/* Create a job and execute it on some set of servers. */
func startRun(args *Arguments) {

    var j Job

    j.Servers = strings.Split(args.Servers, ",")
    j.ServerPort = uint16(args.Port)
    j.RunTime = uint64(args.RunTime)
    j.RampUp = uint64(args.RampUp)
    j.RampDown = uint64(args.RampDown)

    j.Order.JobId = 1
    j.Order.ObjectSize = args.SizeInBytes
    j.Order.Seed = uint64(time.Now().Unix())
    j.Order.GeneratorType = "prng"
    j.Order.RangeStart = 0
    j.Order.RangeEnd = uint64(args.Objects)
    j.Order.Targets = args.Targets

    if args.S3 {
        j.Order.ConnectionType = "s3"
        j.Order.Bucket = args.S3Bucket
        j.Order.Credentials = map[string]string { "access_key": args.S3AccessKey, "secret_key": args.S3SecretKey }
        j.Order.Port = uint16(args.S3Port)
    } else {
        j.Order.ConnectionType = "rados"
        j.Order.Bucket = args.CephPool
        j.Order.Credentials = map[string]string { "username": args.CephUser, "key": args.CephKey }
    }

    m := CreateManager()

    err := m.Run(&j)
    if err != nil {
        fmt.Printf("Error running job: %v\n", err)
    } else {
        fmt.Printf("Done\n")
    }
}

