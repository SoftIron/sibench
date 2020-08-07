package main


/* Singleton instance */
var globalConfig Config


/*
 * All the configuration parameters that we may need.
 *
 * These are not thread-safe: we are relying on the fact that we only ever
 * set the values in main, and then only read them after that.
 */
type Config struct {
    ListenPort uint16
    MountsDir string
}
