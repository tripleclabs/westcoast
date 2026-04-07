package cluster_test

// This file exists solely to import grpctransport so its init() runs,
// registering the default transport factory for cluster package tests.
import _ "github.com/tripleclabs/westcoast/src/actor/cluster/grpctransport"
