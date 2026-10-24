# Formula golden tests

`functions.tsv` lists formulas and their expected display text. The
fixture they refer to is built by `internal/formula/golden`.

The expected values were written by hand from Excel's documented
behavior. To check them against real Excel:

```bash
go run ./tools/goldenxlsx          # writes testdata/golden/excel-check.xlsx
```

1. Open `excel-check.xlsx` in Excel (it recalculates on open).
2. Save it as `testdata/golden/excel-saved.xlsx` (Excel Workbook format).
3. Run `go test ./internal/formula -run TestExcelGolden -v`.

The test reads the values Excel calculated and reports every case where
Excel disagrees with `functions.tsv`. Cases that depend on their own
position (`ROW()`, `COLUMN()`) are skipped. Without `excel-saved.xlsx`
the test is skipped.
