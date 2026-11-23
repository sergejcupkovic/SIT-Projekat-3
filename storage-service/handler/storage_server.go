package handler

import storagepb "stashr/proto/storage"

// StorageServer is the single implementation of storagepb.StorageServiceServer
// that gets registered with gRPC. The rpcs are split by topic: folder rpcs are
// answered by FolderHandler, file rpcs (upload, quota) by FileHandler.
//
// Only FolderHandler embeds storagepb.UnimplementedStorageServiceServer (it
// supplies mustEmbedUnimplementedStorageServiceServer); FileHandler deliberately
// does not, so that no method name is ambiguous between the two.
type StorageServer struct {
	*FolderHandler
	*FileHandler
}

var _ storagepb.StorageServiceServer = StorageServer{}

func NewStorageServer(folders *FolderHandler, files *FileHandler) StorageServer {
	return StorageServer{FolderHandler: folders, FileHandler: files}
}
