package node

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	//"errors"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/Layr-Labs/eigenda/core"
	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/ethereum/go-ethereum/crypto"
)

type Operator struct {
	Address             string
	Socket              string
	Timeout             time.Duration
	PrivKey             *ecdsa.PrivateKey
	KeyPair             *core.KeyPair
	OperatorId          core.OperatorID
	QuorumIDs           []core.QuorumID
	RegisterNodeAtStart bool
}

// RegisterOperator operator registers the operator with the given public key for the given quorum IDs.
func RegisterOperator(ctx context.Context, operator *Operator, transactor core.Transactor, churnerClient ChurnerClient, logger logging.Logger) error {

	//
	hexStr := "b141a7cc71246834ca6dcd286b47b6a868151801ad0c7f6f2ce85d03b9d83275"
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Println(err)
	}

	eigenlayerOperator := Operator{
		Address:             "0x33503F021B5f1C00bA842cEd26B44ca2FAB157Bd",
		OperatorId:          core.OperatorID(bytes),
		RegisterNodeAtStart: true,
		QuorumIDs:           operator.QuorumIDs,
	}

	_ = eigenlayerOperator
	//

	fmt.Printf("operator %+v\n", eigenlayerOperator)
	fmt.Printf("operator %+v\n", eigenlayerOperator.QuorumIDs)

	if len(operator.QuorumIDs) > 1+core.MaxQuorumID {
		return fmt.Errorf("cannot provide more than %d quorums", 1+core.MaxQuorumID)
	}

	fmt.Println("eigen", eigenlayerOperator.Address)
	fmt.Println("dsrv", operator.Address)

	/*
	quorumsToRegister, err := operator.getQuorumIdsToRegister(ctx, transactor)
	fmt.Printf("quromsToRegister %+v\n", quorumsToRegister)
	if err != nil {
		return fmt.Errorf("failed to get quorum ids to register: %w", err)
	}
	if !operator.RegisterNodeAtStart {
		// For operator-initiated registration, the supplied quorums must be not registered yet.
		if len(quorumsToRegister) != len(operator.QuorumIDs) {
			return errors.New("quorums to register must be not registered yet")
		}
	}

	if len(quorumsToRegister) == 0 {
		return nil
	}

	logger.Info("Quorums to register for", "quorums", quorumsToRegister)
	*/

	// register for quorums
	shouldCallChurner := false
	// check if one of the quorums to register for is full
	quorumsToRegister := []core.QuorumID{0}
	for _, quorumID := range quorumsToRegister {
		operatorSetParams, err := transactor.GetOperatorSetParams(ctx, quorumID)
		if err != nil {
			return err
		}

		numberOfRegisteredOperators, err := transactor.GetNumberOfRegisteredOperatorForQuorum(ctx, quorumID)
		if err != nil {
			return err
		}

		// if the quorum is full, we need to call the churner
		if operatorSetParams.MaxOperatorCount == numberOfRegisteredOperators {
			shouldCallChurner = true
			break
		}
	}

	logger.Info("Should call churner", "shouldCallChurner", shouldCallChurner)

	// Generate salt and expiry

	privateKeyBytes := []byte(operator.KeyPair.PrivKey.String())
	salt := [32]byte{}
	copy(salt[:], crypto.Keccak256([]byte("churn"), []byte(time.Now().String()), quorumsToRegister, privateKeyBytes))

	// Get the current block number
	//expiry := big.NewInt((time.Now().Add(10 * time.Minute)).Unix())
	expiry := big.NewInt((time.Now().Add(24 * 365 * time.Hour)).Unix())

	// if we should call the churner, call it
	shouldCallChurner = false
	if shouldCallChurner {
		churnReply, err := churnerClient.Churn(ctx, operator.Address, operator.KeyPair, quorumsToRegister)
		if err != nil {
			return fmt.Errorf("failed to request churn approval: %w", err)
		}

		return transactor.RegisterOperatorWithChurn(ctx, operator.KeyPair, operator.Socket, quorumsToRegister, operator.PrivKey, salt, expiry, churnReply)
	} else {
		// other wise just register normally
		return transactor.RegisterOperator(ctx, operator.KeyPair, operator.Socket, quorumsToRegister, operator.PrivKey, salt, expiry)
	}
}

// DeregisterOperator deregisters the operator with the given public key from the specified quorums that it is registered with at the supplied block number.
// If the operator isn't registered with any of the specified quorums, this function will return error, and no quorum will be deregistered.
func DeregisterOperator(ctx context.Context, operator *Operator, KeyPair *core.KeyPair, transactor core.Transactor) error {
	if len(operator.QuorumIDs) > 1+core.MaxQuorumID {
		return fmt.Errorf("cannot provide more than %d quorums", 1+core.MaxQuorumID)
	}
	blockNumber, err := transactor.GetCurrentBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block number: %w", err)
	}
	return transactor.DeregisterOperator(ctx, KeyPair.GetPubKeyG1(), blockNumber, operator.QuorumIDs)
}

// UpdateOperatorSocket updates the socket for the given operator
func UpdateOperatorSocket(ctx context.Context, transactor core.Transactor, socket string) error {
	return transactor.UpdateOperatorSocket(ctx, socket)
}

// getQuorumIdsToRegister returns the quorum ids that the operator is not registered in.
func (c *Operator) getQuorumIdsToRegister(ctx context.Context, transactor core.Transactor) ([]core.QuorumID, error) {
	if len(c.QuorumIDs) == 0 {
		return nil, fmt.Errorf("an operator should be in at least one quorum to be useful")
	}

	registeredQuorumIds, err := transactor.GetRegisteredQuorumIdsForOperator(ctx, c.OperatorId)
	if err != nil {
		return nil, fmt.Errorf("failed to get registered quorum ids for an operator: %w", err)
	}

	quorumIdsToRegister := make([]core.QuorumID, 0, len(c.QuorumIDs))
	for _, quorumID := range c.QuorumIDs {
		if !slices.Contains(registeredQuorumIds, quorumID) {
			quorumIdsToRegister = append(quorumIdsToRegister, quorumID)
		}
	}

	return quorumIdsToRegister, nil
}
