# IBKR C++ API Source

This directory should contain the official IBKR C++ API source code.

## Setup

1. Download the TWS API from https://interactivebrokers.github.io/
2. Choose "TWS API Latest" → Unix/Mac
3. Extract the zip so that `IBJts/` is a direct child of this directory:
   ```
   third_party/ibkr/IBJts/source/cppclient/client/  ← C++ source
   third_party/ibkr/IBJts/source/proto/              ← .proto files
   ```
4. Regenerate protobuf headers (if your system protoc differs from IBKR's):
   ```bash
   protoc --proto_path=IBJts/source/proto \
          --cpp_out=IBJts/source/cppclient/client/protobufUnix \
          IBJts/source/proto/*.proto
   ```
5. Build the static library:
   ```bash
   cd ../cgo && make
   ```

## License

The IBKR C++ API is under the IB API Non-Commercial License and cannot be redistributed.
This directory is `.gitignore`d.
