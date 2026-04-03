fn main() {
    let proto_file = "proto/service_event.proto";
    println!("cargo:rerun-if-changed={proto_file}");

    let protoc_path = protoc_bin_vendored::protoc_bin_path().expect("find vendored protoc");

    let mut config = prost_build::Config::new();
    config.protoc_executable(protoc_path);

    config
        .compile_protos(&[proto_file], &["proto"])
        .expect("compile service event protobuf");
}
