// pathfinder-c1-frozen-check exercises the ingest preparation path against
// a read-only frozen UDM-Pro archive. No receiver, PostgreSQL or RAM writes.
package main

import (
    "flag"
    "fmt"
    "io"
    "os"
    "time"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorc1"
    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

func main(){
    if err:=run(os.Args[1:],os.Stdout);err!=nil{
        fmt.Fprintln(os.Stderr,"PATHFINDER C1 FROZEN CHECK FAIL:",err)
        os.Exit(1)
    }
}
func run(args []string,output io.Writer)error{
    flags:=flag.NewFlagSet("pathfinder-c1-frozen-check",flag.ContinueOnError)
    flags.SetOutput(io.Discard)
    root:=flags.String("root","","absolute frozen source-archive path (read-only)")
    sensorID:=flags.String("sensor-id","","verified sensor name; no authentication here")
    sensorUUID:=flags.String("sensor-uuid","","existing registered sensor UUID; never generate replacement")
    expectedFiles:=flags.Int("expect-files",0,"expected source JSONL segment count; 0 disables")
    expectedRecords:=flags.Int64("expect-records",0,"expected source record count; 0 disables")
    if err:=flags.Parse(args);err!=nil{return err}
    if flags.NArg()!=0{return fmt.Errorf("unexpected positional arguments")}
    if *root==""||*sensorID==""||*sensorUUID=="" {
        return fmt.Errorf("-root, -sensor-id and -sensor-uuid are required")
    }
    if *expectedFiles<0||*expectedRecords<0{return fmt.Errorf("negative expected counts forbidden")}
    began:=time.Now()
    summary,err:=sensorc1.ReplayFrozen(*root,*sensorID,*sensorUUID,func(p sensorc1.Prepared)error{
        // The replay reader already validated and reprojected original data.
        // This callback never publishes RAM or runs SQL.
        if p.Evidence==nil || p.Evidence.Count==0{return fmt.Errorf("empty prepared evidence")}
        return nil
    })
    if err!=nil{return err}
    if *expectedFiles>0&&summary.Files!=*expectedFiles{
        return fmt.Errorf("frozen source files=%d expected=%d",summary.Files,*expectedFiles)
    }
    if *expectedRecords>0&&summary.Records!=*expectedRecords{
        return fmt.Errorf("frozen source records=%d expected=%d",summary.Records,*expectedRecords)
    }
    sec:=time.Since(began).Seconds()
    rate:=float64(0)
    if sec>0{rate=float64(summary.Records)/sec}
    fmt.Fprintf(output,"PATHFINDER_C1_FROZEN_PREP_PASS files=%d batches=%d records=%d elapsed_seconds=%.3f records_per_sec=%.1f db_writes=0 receiver_changes=0\n",
        summary.Files,summary.Batches,summary.Records,sec,rate)
    for _,stream:=range []sensoringest.Stream{
        sensoringest.Observations,sensoringest.Conntrack,
        sensoringest.Identity,sensoringest.Health,
    }{
        fmt.Fprintf(output,"FROZEN_STREAM %s records=%d\n",stream,summary.ByStream[stream])
    }
    return nil
}
