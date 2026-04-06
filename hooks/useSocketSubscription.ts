import { useEffect } from "react";
import { useSocket } from "../context/SocketContext";

export const useSocketSubscription = (room: string) => {
  const { socket, isConnected } = useSocket();

  useEffect(() => {
    if (!socket || !isConnected || !room) return;

    socket.emit("joinRoom", room);

    return () => {
      socket.emit("leaveRoom", room);
    };
  }, [socket, isConnected, room]);
};
