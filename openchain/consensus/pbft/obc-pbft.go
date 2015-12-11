/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package pbft

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"
)

type ObcPbft struct {
	cpi  consensus.CPI // link to the CPI
	pbft *Plugin
}

var pluginInstance *ObcPbft // Singleton service

// GetPlugin returns the handle to the Plugin singleton and updates
// the CPI if necessary.
func GetPlugin(c consensus.CPI) consensus.Consenter {
	if pluginInstance == nil {
		pluginInstance = NewObcPbft(c)
	} else {
		pluginInstance.cpi = c // otherwise, just update the CPI
	}
	return pluginInstance
}

// NewObcPbft creates a new PBFT instance that provides the OBC
// Consenter interface.  Internally, it uses an opaque `Pbft`
func NewObcPbft(cpi consensus.CPI) (op *ObcPbft) {
	op = &ObcPbft{cpi: cpi}

	// set ID
	address, _ := op.cpi.GetReplicaAddress(true)
	id, _ := op.cpi.GetReplicaID(address[0])
	op.pbft = NewPbft(id, op)
	return
}

// Close tears down all resources
func (op *ObcPbft) Close() {
	op.pbft.Close()
}

// RecvMsg receives both CHAIN_TRANSACTION and CONSENSUS messages from
// the stack.  New transaction requests are broadcast to all replicas,
// so that the current primary will receive the request.
func (op *ObcPbft) RecvMsg(msgWrapped *pb.OpenchainMessage) error {
	if msgWrapped.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")
		// TODO verify transaction
		// if _, err := op.cpi.TransactionPreValidation(...); err != nil {
		//   logger.Warning("Invalid request");
		//   return err
		// }
		op.pbft.Request(msgWrapped.Payload)
		req := &Request{Payload: msgWrapped.Payload}
		msg := &Message{&Message_Request{req}}
		msgRaw, _ := proto.Marshal(msg)
		op.Broadcast(msgRaw)
		return nil
	}
	if msgWrapped.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", msgWrapped.Type)
	}

	pbftMsg := &Message{}
	err := proto.Unmarshal(msgWrapped.Payload, pbftMsg)
	if err != nil {
		return err
	}
	if req := pbftMsg.GetRequest(); req != nil {
		op.pbft.Request(req.Payload)
	} else {
		op.pbft.Receive(msgWrapped.Payload)
	}
	return nil
}

// Execute is called by the inner pbft to execute an opaque request,
// which corresponds to a OBC Transaction.
func (op *ObcPbft) Execute(txRaw []byte) {
	tx := &pb.Transaction{}
	err := proto.Unmarshal(txRaw, tx)
	if err != nil {
		return
	}
	// TODO verify transaction
	// if tx, err = op.cpi.TransactionPreExecution(...); err != nil {
	//   logger.Error("Invalid request");
	// } else {
	// ...
	// }
	// XXX switch to https://github.com/openblockchain/obc-peer/issues/340
	op.cpi.ExecTXs([]*pb.Transaction{tx})
}

// Broadcast is called by the inner pbft to multicast a message to all
// replicas.
func (op *ObcPbft) Broadcast(msg []byte) {
	ocMsg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: msg,
	}
	op.cpi.Broadcast(ocMsg)
}
