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
type Config struct {
    Server bool
    S3 bool
    Run bool
    Bucket string
    Port int
    SibenchPort int
    Size string
    SizeInBytes uint64
    Objects int
    Servers string
    RunTime int
    RampUp int
    RampDown int
    Targets []string
    AccessKey string
    SecretKey string
}


/* Return a usage string for DocOpt argument parsing. */
func usage() string {
    return `SoftIron Benchmark Tool.
Usage:
  sibench server [--sibench-port PORT]
  sibench s3 run [--sibench-port PORT] [-s SIZE] [-o COUNT] [-r TIME] [-u TIME] [-d TIME] [-b BUCKET] [-p PORT]
                 [--servers SERVERS] (-a KEY) (-k KEY) <targets> ...
Options:
  -o COUNT, --objects COUNT      The number of objects to use as our working set  [default: 1000]
  -s SIZE, --size SIZE           Object size to test. [default: 1M]
  -r TIME, --run-time TIME       The time spent on each phase of the benchmark.  [default: 30]
  -u TIME, --ramp-up TIME        The extra time we run at the start of each phase where we don't collect stats.  [default: 5]
  -d TIME, --ramp-down TIME      The extra time we run at the end of each phase where we don't collect stats.  [default: 2]
  -b BUCKET, --bucket BUCKET     The name of the bucket we wish to use for S3 operations.  [default: sibench]
  -p PORT, --port PORT           The port on which to connect to S3.  [default: 7480]
  --sibench-port PORT            The port that servers should listen on.  Only needs to be set if there's a conflict. [default: 5150]
  --servers SERVERS              A comma-separated list of sibench servers to connect to.  [default: localhost]
  --access-key KEY, -a KEY       S3 access key
  --secret-key KEY, -k KEY       S3 secret key
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
func validateConfig(conf *Config) error {
    if (conf.Port < 0) || ( conf.Port > int(math.MaxUint16)) {
        return fmt.Errorf("S3 Port not in range: %v", conf.SibenchPort)
    }

    if (conf.SibenchPort < 0) || ( conf.SibenchPort > int(math.MaxUint16)) {
        return fmt.Errorf("Sibench Port not in range: %v", conf.SibenchPort)
    }

    // Turn the size (in K or M) into bytes...

    re := regexp.MustCompile(`([1-9][0-9]*)([kKmM])`)
    groups := re.FindStringSubmatch(conf.Size)
    if groups == nil {
        return fmt.Errorf("Bad size specifier: %v", conf.Size)
    }

    val, _ := strconv.Atoi(groups[1])
    conf.SizeInBytes = uint64(val) * 1024
    if strings.EqualFold(groups[2], "m") {
        conf.SizeInBytes *= 1024
    }

    return nil
}


func main() {
    // Error should never happen outside of development, since docopt is complaining that our usage string has bad syntax.
    opts, err := docopt.ParseDoc(usage())
    dieOnError(err, "Error parsing arguments")

    // Error should never happen outside of development, since docopt is complaining that our type bindings are wrong.
    var conf Config
    err = opts.Bind(&conf)
    dieOnError(err, "Failure binding config")

    // This can error on bad user input.
    err = validateConfig(&conf)
    dieOnError(err, "Failure validating arguments")

    fmt.Printf("%+v\n", conf)

    if conf.Server {
        startServer(&conf)
    }

    if conf.Run {
        startRun(&conf)
    }
}


/* Start a server, listening on a TCP port */
func startServer(conf *Config) {
    listenPort := uint16(conf.SibenchPort)
    err := StartForeman(listenPort)
    dieOnError(err, "Failure creating server")
}


/* Create a job and execute it on some set of servers. */
func startRun(conf *Config) {

    var j Job

    j.Servers = strings.Split(conf.Servers, ",")
    j.ServerPort = uint16(conf.SibenchPort)
    j.RunTime = uint64(conf.RunTime)
    j.RampUp = uint64(conf.RampUp)
    j.RampDown = uint64(conf.RampDown)

    j.Order.JobId = 1
    j.Order.Bucket = conf.Bucket
    j.Order.ObjectSize = conf.SizeInBytes
    j.Order.Seed = uint64(time.Now().Unix())
    j.Order.GeneratorType = "prng"
    j.Order.RangeStart = 0
    j.Order.RangeEnd = uint64(conf.Objects)
    j.Order.ConnectionType = "s3"
    j.Order.Targets = conf.Targets
    j.Order.Port = uint16(conf.Port)
    j.Order.Credentials = map[string]string { "access_key": conf.AccessKey, "secret_key": conf.SecretKey }

    m := CreateManager()

    err := m.Run(&j)
    if err != nil {
        fmt.Printf("Error running job: %v\n", err)
    } else {
        fmt.Printf("Done\n")
    }
}

