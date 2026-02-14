# PackSpec v1 TCK

Run the PackSpec v1 Test Compatibility Kit (TCK):

```bash
make test-packspec-tck
```

or

```bash
bash scripts/test_packspec_tck.sh ./gait
```

## Fixture Root

- `scripts/testdata/packspec_tck/v1/`

Current base vector:

- `run_record_input.json` (deterministic source for runpack + PackSpec generation)

## TCK Coverage

The TCK validates these required vectors:

1. Valid run pack (`pack_type=run`) verify pass.
2. Valid job pack (`pack_type=job`) verify pass.
3. Tampered hash pack verify fail.
4. Undeclared file pack verify fail.
5. Schema-invalid manifest verify fail.
6. Legacy migration vectors:
   - legacy runpack verify through `gait pack verify`
   - legacy guard pack verify through `gait pack verify`
7. Deterministic `pack diff` output (stable hash across repeated runs).

## Exit Contract

- `0`: full TCK pass.
- non-zero: at least one contract vector failed.

## Contribution Rules

When modifying PackSpec behavior:

- update/add fixture vectors under `scripts/testdata/packspec_tck/v1/`
- keep vectors deterministic and content-addressable
- update this document with any new mandatory vector class
