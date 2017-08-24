package main

import (
	"fmt"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/service/cloudwatch"
    "github.com/jessevdk/go-flags"
    "log"
    "os"
    "strings"
)


type Options struct {
    IsAggregated bool `long:"aggregated" description:"Adds aggregated metrics for instance type, AMI ID, and overall for the region"`
    IsAutoScaling bool `long:"auto-scaling" description:"Adds aggregated metrics for the Auto Scaling group"`
    IsMemUtil bool `long:"mem-util" description:"Memory Utilization(percent)"`
    IsMemUsed bool `long:"mem-used" description:"Memory Used(bytes)"`
    IsMemAvail bool `long:"mem-avail" description:"Memory Available(bytes)"`
    IsSwapUtil bool `long:"swap-util" description:"Swap Utilization(percent)"`
    IsSwapUsed bool `long:"swap-used" description:"Swap Used(bytes)"`
    IsDiskSpaceUtil bool `long:"disk-space-util" description:"Disk Space Utilization(percent)"`
    IsDiskSpaceUsed bool `long:"disk-space-used" description:"Disk Space Used(bytes)"`
    IsDiskSpaceAvail bool `long:"disk-space-avail" description:"Disk Space Available(bytes)"`
    IsDiskInodeUtil bool `long:"disk-inode-util" description:"Disk Inode Utilization(percent)"`

    NameSpace string `long:"namespace" default:"Linux/System" description:"CloudWatch metric namespace"`
    DiskPaths string `long:"disk-path" default:"/" description:"Disk Path"`

	IsDryRun bool `short:"d" long:"dry-run" description:"Marks this a dry run. Does not attempt to contact aws, prints payload to stdout."`
}

var opts Options
var parser = flags.NewParser(&opts, flags.Default)


func main() {
    if _, err := parser.Parse(); err != nil {
        if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
            os.Exit(0)
        } else {
            log.Fatal("Parsing command line options failed.")
        }
    }

    var metadata map[string]string
	if !opts.IsDryRun {
		temp_metadata, err := getInstanceMetadata()
        if err != nil {
            log.Fatal("Can't get InstanceData, please confirm we are running on a AWS EC2 instance: ", err)
        }
        metadata = temp_metadata
	} else {
		metadata = map[string]string{
            "region": "us-east-1",
            "imageId": "i-fakefakefake",
            "instanceType": "r3.fake",
		}
	}

    memUtil, memUsed, memAvail, swapUtil, swapUsed, err := memoryUsage()

    var metricData []*cloudwatch.MetricDatum

    var dims []*cloudwatch.Dimension
    if !opts.IsAggregated && !opts.IsDryRun {
        dims = getDimensions(metadata)
    }

    if opts.IsAutoScaling && !opts.IsDryRun {
        if as, err := getAutoscalingGroup(metadata["instanceId"], metadata["region"]); as != nil && err == nil {
            dims = append(dims, &cloudwatch.Dimension{
                Name:  aws.String("AutoScalingGroupName"),
                Value: as,
            })
        }
        if err != nil {
            log.Fatal(err)
        }
    }

    if opts.IsMemUtil {
        metricData, err = addMetric("MemoryUtilization", "Percent", memUtil, dims, metricData)
        if err != nil {
            log.Fatal("Can't add memory usage metric: ", err)
        }
    }

    if opts.IsMemUsed {
        metricData, err = addMetric("MemoryUsed", "Bytes", memUsed, dims, metricData)
        if err != nil {
            log.Fatal("Can't add memory used metric: ", err)
        }
    }
    if opts.IsMemAvail {
        metricData, err = addMetric("MemoryAvail", "Bytes", memAvail, dims, metricData)
        if err != nil {
            log.Fatal("Can't add memory available metric: ", err)
        }
    }
    if opts.IsSwapUsed {
        metricData, err = addMetric("SwapUsed", "Bytes", swapUsed, dims, metricData)
        if err != nil {
            log.Fatal("Can't add swap used metric: ", err)
        }
    }
    if opts.IsSwapUtil {
        metricData, err = addMetric("SwapUtil", "Percent", swapUtil, dims, metricData)
        if err != nil {
            log.Fatal("Can't add swap usage metric: ", err)
        }
    }

    paths := strings.Split(opts.DiskPaths, ",")

    for _, val := range paths {
        diskspaceUtil, diskspaceUsed, diskspaceAvail, diskinodesUtil, err := DiskSpace(val)
        if err != nil {
            log.Fatal("Can't get DiskSpace %s", err)
        }
        metadata["fileSystem"] = val

        var dims []*cloudwatch.Dimension
        if !opts.IsAggregated {
            dims = getDimensions(metadata)
        }

        if opts.IsAutoScaling {
            if as, err := getAutoscalingGroup(metadata["instanceId"], metadata["region"]); as != nil && err == nil {
                dims = append(dims, &cloudwatch.Dimension{
                    Name:  aws.String("AutoScalingGroupName"),
                    Value: as,
                })
            }
            if err != nil {
                log.Fatal(err)
            }
        }

        if opts.IsDiskSpaceUtil {
            metricData, err = addMetric("DiskUtilization", "Percent", diskspaceUtil, dims, metricData)
            if err != nil {
                log.Fatal("Can't add Disk Utilization metric: ", err)
            }
        }
        if opts.IsDiskSpaceUsed {
            metricData, err = addMetric("DiskUsed", "Bytes", float64(diskspaceUsed), dims, metricData)
            if err != nil {
                log.Fatal("Can't add Disk Used metric: ", err)
            }
        }
        if opts.IsDiskSpaceAvail {
            metricData, err = addMetric("DiskAvail", "Bytes", float64(diskspaceAvail), dims, metricData)
            if err != nil {
                log.Fatal("Can't add Disk Available metric: ", err)
            }
        }
        if opts.IsDiskInodeUtil {
            metricData, err = addMetric("DiskInodesUtilization", "Percent", diskinodesUtil, dims, metricData)
            if err != nil {
                log.Fatal("Can't add Disk Inodes Utilization metric: ", err)
            }
        }
    }

    if opts.IsDryRun {
        fmt.Printf("Dry run. metric data that would be sent:\n")
        for _, datum := range metricData {
            fmt.Printf(datum.GoString())
        }
    } else {
    err = putMetric(metricData, opts.NameSpace, metadata["region"])
        if err != nil {
            log.Fatal("Can't put CloudWatch Metric")
        }
    }
}
