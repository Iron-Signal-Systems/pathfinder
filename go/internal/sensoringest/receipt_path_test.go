package sensoringest

import (
    "os"
    "path/filepath"
    "testing"
)

func TestCustodyReceiptPath(t *testing.T) {
    store, err := OpenCustody(filepath.Join(t.TempDir(), "private"))
    if err != nil { t.Fatal(err) }
    batch := goodBatch()
    receipt, err := store.Preserve(batch)
    if err != nil { t.Fatal(err) }
    path, err := store.ReceiptPath(receipt)
    if err != nil { t.Fatal(err) }
    info, err := os.Lstat(path)
    if err != nil { t.Fatal(err) }
    if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
        t.Fatalf("unsecured receipt custody %s: %v", path, info.Mode())
    }
    receipt.BatchID = "../malicious"
    if _, err := store.ReceiptPath(receipt); err == nil {
        t.Fatal("expected malformed receipt identity rejection")
    }
}
