// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main


/* Singleton instance */
var globalConfig Config


/*
 * All the configuration parameters that we may need to run a server.
 *
 * These are not thread-safe: we are relying on the fact that we only ever
 * set the values in main, and then only read them after that.
 */
type Config struct {
    ListenPort uint16
    MountsDir string
}
