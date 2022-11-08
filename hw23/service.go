package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	status "google.golang.org/grpc/status"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

const (
	LogRotateInterval  = 300 * time.Microsecond
	StatRotateInterval = 10 * time.Millisecond
	StatDepth          = 10
)

func StartMyMicroservice(ctx context.Context, listenAddr, ACLData string) error {

	adminManager, err := NewAdminManager(ctx, ACLData, listenAddr)
	if err != nil {
		return err
	}
	bizManager, err := NewBizManager(ctx, adminManager)
	if err != nil {
		return err
	}

	unaryInterceptor := func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		err := adminManager.ProcessRequest(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
	streamInterceptor := func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		err := adminManager.ProcessRequest(ss.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		return handler(srv, ss)
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(unaryInterceptor),
		grpc.StreamInterceptor(streamInterceptor),
	)
	RegisterBizServer(server, bizManager)
	RegisterAdminServer(server, adminManager)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Panicln("can't listen addr", listenAddr, err)
	}

	go func(ctx context.Context, server *grpc.Server) {
		for range ctx.Done() {
		}
		server.GracefulStop()
	}(ctx, server)

	go func(server *grpc.Server) {
		err = server.Serve(lis)
		if err != nil {
			log.Panicln("Server stop", err)
		}
	}(server)

	return nil
}

type BizManager struct {
	ctx context.Context
	adm *AdminManager

	UnimplementedBizServer
}

func NewBizManager(ctx context.Context, adm *AdminManager) (*BizManager, error) {
	return &BizManager{ctx: ctx, adm: adm}, nil
}

func (bizManager *BizManager) Check(ctx context.Context, in *Nothing) (*Nothing, error) {
	return in, nil
}
func (bizManager *BizManager) Add(ctx context.Context, in *Nothing) (*Nothing, error) {
	return in, nil
}
func (bizManager *BizManager) Test(ctx context.Context, in *Nothing) (*Nothing, error) {
	return in, nil
}

type AdminManager struct {
	ctx            context.Context
	acl            map[string][]string
	muChan         *sync.Mutex
	chanList       []chan *Event
	activeChanList []bool

	UnimplementedAdminServer
}

func NewAdminManager(ctx context.Context, aclStr, addr string) (*AdminManager, error) {
	acl := map[string][]string{}
	err := json.Unmarshal([]byte(aclStr), &acl)
	if err != nil {
		return nil, err
	}
	adminManager := &AdminManager{ctx: ctx, acl: acl, muChan: new(sync.Mutex)}

	return adminManager, nil
}

func (adminManager *AdminManager) Logging(in *Nothing, out Admin_LoggingServer) error {
	ch, chIdx := adminManager.AquireChannel()
	defer adminManager.ReleaseChannel(chIdx)
	for {
		select {
		case <-adminManager.ctx.Done():
			return nil

		case <-out.Context().Done():
			return nil

		case event := <-ch:
			err := out.Send(event)
			if err != nil {
				return err
			}
		}
	}
}
func (adminManager *AdminManager) Statistics(in *StatInterval, out Admin_StatisticsServer) error {
	interval := in.IntervalSeconds
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	ch, chIdx := adminManager.AquireChannel()
	defer adminManager.ReleaseChannel(chIdx)
	defer ticker.Stop()
	stat := new(Stat)
	stat.Timestamp = time.Now().Unix()
	stat.ByMethod = map[string]uint64{}
	stat.ByConsumer = map[string]uint64{}
	for {
		select {
		case <-adminManager.ctx.Done():
			return nil

		case <-out.Context().Done():
			return nil

		case event := <-ch:
			stat.ByConsumer[event.Consumer]++
			stat.ByMethod[event.Method]++

		case <-ticker.C:
			err := out.Send(stat)
			if err != nil {
				return err
			}
			stat.Timestamp = time.Now().Unix()
			stat.ByMethod = map[string]uint64{}
			stat.ByConsumer = map[string]uint64{}
		}
	}
}

func (adminManager *AdminManager) ProcessRequest(ctx context.Context, fullMethod string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "Retrieving metadata is failed")
	}
	consumer, ok := md["consumer"]
	if !ok || len(consumer) == 0 {
		return status.Errorf(codes.Unauthenticated, "Consumer not found")
	}

	allowed, ok := adminManager.acl[consumer[0]]
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Consumer not found")
	}
	isAllowed := false
	for _, rule := range allowed {
		if rule[len(rule)-1] == '*' {
			if strings.HasPrefix(fullMethod, rule[:len(rule)-1]) {
				isAllowed = true
				break
			}
		} else {
			if rule == fullMethod {
				isAllowed = true
				break
			}
		}
	}
	if !isAllowed {
		return status.Errorf(codes.Unauthenticated, "Consumer not allowed")
	}

	client, _ := peer.FromContext(ctx)
	logRecord := new(Event)
	logRecord.Consumer = consumer[0]
	logRecord.Timestamp = time.Now().Unix()
	logRecord.Method = fullMethod
	logRecord.Host = client.Addr.String()

	adminManager.muChan.Lock()
	for i := range adminManager.chanList {
		if adminManager.activeChanList[i] {
			time.Sleep(10 * time.Microsecond)
			adminManager.chanList[i] <- logRecord
		}
	}
	adminManager.muChan.Unlock()

	return nil
}

func (adminManager *AdminManager) AquireChannel() (chan *Event, int) {
	adminManager.muChan.Lock()
	defer adminManager.muChan.Unlock()
	for i, ok := range adminManager.activeChanList {
		if !ok {
			adminManager.chanList[i] = make(chan *Event)
			adminManager.activeChanList[i] = true
			return adminManager.chanList[i], i
		}
	}
	c := make(chan *Event)
	adminManager.chanList = append(adminManager.chanList, c)
	adminManager.activeChanList = append(adminManager.activeChanList, true)
	return c, len(adminManager.activeChanList) - 1
}
func (adminManager *AdminManager) ReleaseChannel(idx int) {
	adminManager.muChan.Lock()
	adminManager.activeChanList[idx] = false
	close(adminManager.chanList[idx])
	adminManager.chanList[idx] = nil
	adminManager.muChan.Unlock()
}
