pub mod serviceevent {
    pub mod v1 {
        include!(concat!(env!("OUT_DIR"), "/serviceevent.v1.rs"));
    }
}
