package synapse

import (
	"github.com/streadway/amqp"
	"github.com/bitly/go-simplejson"
	"fmt"
	"encoding/json"
)

/**
绑定RPC监听队列
 */
func (s *Server) serverQueue() *amqp.Channel {
	channel := s.CreateChannel(s.RpcProcessNum, "RpcServer")
	q, err := channel.QueueDeclare(
		fmt.Sprintf("%s_%s_server", s.SysName, s.AppName), // name
		true,                                              // durable
		true,                                              // delete when usused
		false,                                             // exclusive
		false,                                             // no-wait
		nil,                                               // arguments
	)
	if err != nil {
		Log(fmt.Sprintf("Failed to declare Rpc Queue: %s", err), LogError)
	}

	err = channel.QueueBind(
		q.Name,
		fmt.Sprintf("server.%s", s.AppName),
		s.SysName,
		false,
		nil)
	if err != nil {
		Log(fmt.Sprintf("Failed to Bind Rpc Exchange and Queue: %s", err), LogError)
	}
	return channel
}

/**
创建RPC监听
callback回调为监听到RPC请求后的处理函数
 */
func (s *Server) rpcServer(channel *amqp.Channel) {
	msgs, err := channel.Consume(
		fmt.Sprintf("%s_%s_server", s.SysName, s.AppName), // queue
		fmt.Sprintf("%s.%s.server.%s", s.SysName, s.AppName, s.AppId),
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		Log(fmt.Sprintf("Failed to register Rpc Server consumer: %s", err), LogError)
	}
	for d := range msgs {
		go s.rpcHandler(d, channel)
	}
}

/**
RPC请求处理器
 */
func (s *Server) rpcHandler(d amqp.Delivery, channel *amqp.Channel) {
	query, _ := simplejson.NewJson(d.Body)
	if s.Debug {
		logData, _ := query.MarshalJSON()
		Log(fmt.Sprintf("RPC Receive: (%s)%s->%s@%s %s", d.MessageId, d.ReplyTo, d.Type, s.AppName, logData), LogDebug)
	}
	callback, ok := s.RpcCallback[d.Type]
	result, _ := json.Marshal(map[string]string{"rpc_error": "method not found"})
	if ok {
		result, _ = json.Marshal(callback(query, d))
	}
	reply := fmt.Sprintf("client.%s.%s", d.ReplyTo, d.AppId)
	err = channel.Publish(
		s.SysName, // exchange
		reply,     // routing key
		false,     // mandatory
		false,     // immediatec
		amqp.Publishing{
			AppId:         s.AppId,
			MessageId:     s.randomString(20),
			ReplyTo:       s.AppName,
			Type:          d.Type,
			CorrelationId: d.MessageId,
			Body:          result,
		})
	if s.Debug {
		Log(fmt.Sprintf("RPC Return: (%s)%s@%s->%s %s", d.MessageId, d.Type, s.AppName, d.ReplyTo, result), LogDebug)
	}
	if err != nil {
		Log(fmt.Sprintf("Failed to reply Rpc Request: %s", err), LogError)
	}
	d.Ack(false)
}
