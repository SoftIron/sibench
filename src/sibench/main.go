package main

import "encoding/json"
import "github.com/docopt/docopt-go"
import "fmt"
import "math"
import "os"
import "strings"


type Config struct {
    Server bool
    S3 bool
    Run bool
    SibenchPort int
    Size string
    Objects int
    Servers string
    RunTime int
    RampUp int
    RampDown int
    Targets []string
    AccessKey string
    SecretKey string
}


/* Return a usage string for DocOpt argument parsing.:w
 */
func usage() string {
    return `SoftIron Benchmark Tool.
Usage:
  sibench server [--sibench-port PORT]
  sibench s3 run [--sibench-port PORT] [-s SIZE] [-o COUNT] [-r TIME] [-u TIME] [-d TIME] [--servers SERVERS] (-a KEY) (-k KEY) <targets> ...
Options:
  -o COUNT, --objects COUNT      The number of objects to use as our working set  [default: 1000]
  -s SIZE, --size SIZE           Object size to test. [default: 1M]
  -r TIME, --run-time TIME       The time spent on each phase of the benchmark.  [default: 30]
  -u TIME, --ramp-up TIME        The extra time we run at the start of each phase where we don't collect stats.  [default: 5]
  -d TIME, --ramp-down TIME      The extra time we run at the end of each phase where we don't collect stats.  [default: 2]
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


func dieOnError(err error, format string, a ...interface{}) {
    if err != nil {
        fmt.Printf(format, a)
        fmt.Printf(": %v\n", err)
        os.Exit(-1)
    }
}


func validateConfig(conf *Config) error {
    if (conf.SibenchPort < 0) || ( conf.SibenchPort > int(math.MaxUint16)) {
        return fmt.Errorf("Sibench Port not in range: %v", conf.SibenchPort)
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

    fmt.Printf("%+v\n", conf)

    // This can error on bad user input.
    err = validateConfig(&conf)
    dieOnError(err, "Failure validating arguments")

    if conf.Server {
        startServer(&conf)
    }

    if conf.Run {
        startRun(&conf)
    }
}


func startServer(conf *Config) {
    listenPort := uint16(conf.SibenchPort)
    err := StartForeman(listenPort)
    dieOnError(err, "Failure creating server")
}


func startRun(conf *Config) {

    var j Job

    j.Servers = strings.Split(conf.Servers, ",")
    j.ServerPort = uint16(conf.SibenchPort)
    j.RunTime = uint64(conf.RunTime)
    j.RampUp = uint64(conf.RampUp)
    j.RampDown = uint64(conf.RampDown)

    j.Order.JobId = 1
    j.Order.Bucket = "sibench"
    j.Order.ObjectSize = 1024 * 1024
    j.Order.Seed = 12345
    j.Order.GeneratorType = "prng"
    j.Order.RangeStart = 0
    j.Order.RangeEnd = uint64(conf.Objects)
    j.Order.ConnectionType = "s3"
    j.Order.Targets = conf.Targets
    j.Order.Port = 7480
    j.Order.Credentials = map[string]string { "access_key": conf.AccessKey, "secret_key": conf.SecretKey }

    m := CreateManager()

    err := m.Run(&j)
    if err != nil {
        fmt.Printf("Error running job: %v\n", err)
    } else {
        fmt.Printf("Done\n")
    }
}

