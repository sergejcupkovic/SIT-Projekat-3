package handler

import storagepb "stashr/proto/storage"

// StorageServer joins the folder and file handlers into one gRPC server.
type StorageServer struct {
	*FolderHandler
	*FileHandler
}

var _ storagepb.StorageServiceServer = StorageServer{}

func NewStorageServer(folders *FolderHandler, files *FileHandler) StorageServer {
	return StorageServer{FolderHandler: folders, FileHandler: files}
}
