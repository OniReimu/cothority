package cosi

import (
	"fmt"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
)

var testSuite = edwards.NewAES128SHA256Ed25519(false)

// TestCosiCommitment test if the commitment generation is correct
func TestCosiCommitment(t *testing.T) {
	var length = 5
	cosis := genCosis(length)
	commitments := genCommitments(cosis)
	root := genCosi()
	root.Commit(commitments)
	// compute the aggregate commitment ourself...
	aggCommit := testSuite.Point().Null()
	// add commitment of children
	for _, com := range commitments {
		aggCommit = aggCommit.Add(aggCommit, com.Commitment)
	}
	// add commitment of root
	aggCommit = aggCommit.Add(aggCommit, root.commitment)
	if !aggCommit.Equal(root.aggregateCommitment) {
		t.Fatal("Aggregate Commitment are not equal")
	}
}

func TestCosiChallenge(t *testing.T) {
	root, children := genPostCommitmentPhaseCosi(5)
	msg := []byte("Hello World Cosi\n")
	chal, err := root.CreateChallenge(msg)
	if err != nil {
		t.Fatal("Error during challenge generation")
	}
	for _, child := range children {
		child.Challenge(chal)
		if !child.challenge.Equal(chal.Challenge) {
			t.Fatal("Error during challenge on children")
		}
	}
}

// TestCosiResponse will test wether the response generation is correct or not
func TestCosiResponse(t *testing.T) {
	msg := []byte("Hello World Cosi")
	// go to the challenge phase
	root, children := genPostChallengePhaseCosi(5, msg)
	var responses []*Response

	// for verification later
	aggResponse := testSuite.Secret().Zero()
	for _, ch := range children {
		// generate the response of each children
		r, err := ch.CreateResponse()
		if err != nil {
			t.Fatal("Error creating response:", err)
		}
		responses = append(responses, r)
		aggResponse = aggResponse.Add(aggResponse, r.Response)
	}
	// pass them up to the root
	_, err := root.Response(true, responses)
	if err != nil {
		t.Fatal("Response phase failed:", err)
	}

	// verify it
	aggResponse = aggResponse.Add(aggResponse, root.response)
	if !aggResponse.Equal(root.aggregateResponse) {
		t.Fatal("Responses aggregated not equal")
	}
}

// TestCosiVerifyResponse test if the response generation is correct and if we
// can verify the final signature.
func TestCosiVerifyResponse(t *testing.T) {
	msg := []byte("Hello World Cosi")
	root, children, err := genFinalCosi(5, msg)
	if err != nil {
		t.Fatal(err)
	}
	aggregatedPublic := testSuite.Point().Null()
	for _, ch := range children {
		// add children public key
		aggregatedPublic = aggregatedPublic.Add(aggregatedPublic, testSuite.Point().Mul(nil, ch.private))
	}
	// add root public key
	aggregatedPublic = aggregatedPublic.Add(aggregatedPublic, testSuite.Point().Mul(nil, root.private))
	// verify the responses / commitment
	if err := root.VerifyResponses(aggregatedPublic); err != nil {
		t.Fatal("Verification of responses / commitment has failed:", err)
	}

	// recompute the challenge and check if it is the same
	commitment := testSuite.Point()
	commitment = commitment.Add(commitment.Mul(nil, root.aggregateResponse), testSuite.Point().Mul(aggregatedPublic, root.challenge))
	// T is the recreated V_hat
	T := testSuite.Point().Null()
	T = T.Add(T, commitment)

	pb, err := T.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	cipher := testSuite.Cipher(pb)
	cipher.Message(nil, nil, msg)
	// reconstructed challenge
	challenge := testSuite.Secret().Pick(cipher)

	if !challenge.Equal(root.challenge) {
		t.Fatal("Root challenge != challenge recomputed")
	}
	if !challenge.Equal(children[0].challenge) {
		t.Fatal("Children[0] challenge != challenge recomputed")
	}
	if err := VerifySignature(testSuite, msg, aggregatedPublic, root.challenge, root.aggregateResponse); err != nil {
		t.Fatal("Error veriying:", err)
	}
}

func TestVerify(t *testing.T) {
	msg := []byte("Exceptions")
	p1 := config.NewKeyPair(network.Suite)
	p2 := config.NewKeyPair(network.Suite)
	agg := network.Suite.Point().Add(p1.Public, p2.Public)
	c1 := NewCosi(network.Suite, p1.Secret)
	c2 := NewCosi(network.Suite, p2.Secret)
	c2.Announce(c1.CreateAnnouncement())
	c1.Commit([]*Commitment{c2.CreateCommitment()})
	ch, err := c1.CreateChallenge(msg)
	dbg.ErrFatal(err)
	c2.Challenge(ch)
	re, err := c2.CreateResponse()
	dbg.ErrFatal(err)
	_, err = c1.Response(true, []*Response{re})
	dbg.ErrFatal(err)

	dbg.ErrFatal(VerifySignature(network.Suite, msg, agg, c1.challenge, c1.aggregateResponse))
}

func TestVerifyWithException(t *testing.T) {
	msg := []byte("Hello World Cosi")
	root, children, err := genFinalCosi(5, msg)
	if err != nil {
		t.Fatal(err)
	}
	aggregatedPublic := testSuite.Point().Null()
	for _, ch := range children {
		// add children public key
		aggregatedPublic = aggregatedPublic.Add(aggregatedPublic, testSuite.Point().Mul(nil, ch.private))
	}
	// add root public key
	rootPub := testSuite.Point().Mul(nil, root.private)
	aggregatedPublic = aggregatedPublic.Add(aggregatedPublic, rootPub)

	ex := []Exception{{rootPub, root.commitment}}
	response := root.aggregateResponse
	if VerifySignatureWithException(testSuite, aggregatedPublic, msg,
		root.challenge, response, ex) == nil {
		t.Fatal("This should fail")
	}

	response = testSuite.Secret().Sub(root.aggregateResponse, root.response)
	dbg.ErrFatal(VerifySignatureWithException(testSuite, aggregatedPublic, msg,
		root.challenge, response, ex))
}

func TestVerifyWithException2(t *testing.T) {
	msg := []byte("Exceptions")
	p1 := config.NewKeyPair(network.Suite)
	p2 := config.NewKeyPair(network.Suite)
	agg := network.Suite.Point().Add(p1.Public, p2.Public)
	c1 := NewCosi(network.Suite, p1.Secret)
	c2 := NewCosi(network.Suite, p2.Secret)
	c2.Announce(c1.CreateAnnouncement())
	c1.Commit([]*Commitment{c2.CreateCommitment()})
	ch, err := c1.CreateChallenge(msg)
	dbg.ErrFatal(err)
	c2.Challenge(ch)
	re, err := c2.CreateResponse()
	dbg.ErrFatal(err)
	_, err = c1.Response(false, []*Response{re})
	dbg.ErrFatal(err)

	if VerifySignature(network.Suite, msg, agg, c1.challenge, c1.aggregateResponse) == nil {
		t.Fatal("This shouldn't be verifiable")
	}
	ex := []Exception{{p1.Public, c1.commitment}}
	dbg.ErrFatal(VerifySignatureWithException(testSuite, agg, msg,
		c1.challenge, c1.aggregateResponse, ex))
}

func genKeyPair(nb int) []*config.KeyPair {
	var kps []*config.KeyPair
	for i := 0; i < nb; i++ {
		kps = append(kps, config.NewKeyPair(testSuite))
	}
	return kps
}

func genCosi() *Cosi {
	kp := config.NewKeyPair(testSuite)
	return NewCosi(testSuite, kp.Secret)
}
func genCosis(nb int) []*Cosi {
	kps := genKeyPair(nb)
	var cosis []*Cosi
	for _, kp := range kps {
		cosis = append(cosis, NewCosi(testSuite, kp.Secret))
	}
	return cosis
}

func genCommitments(cosis []*Cosi) []*Commitment {
	commitments := make([]*Commitment, len(cosis))
	for i := range cosis {
		commitments[i] = cosis[i].CreateCommitment()
	}
	return commitments
}

// genPostCommitmentPhaseCosi returns the Root and its Children Cosi. They have
// already made the Commitment phase.
func genPostCommitmentPhaseCosi(nb int) (*Cosi, []*Cosi) {
	cosis := genCosis(nb)
	commitments := genCommitments(cosis)
	root := genCosi()
	root.Commit(commitments)
	return root, cosis
}

func genPostChallengePhaseCosi(nb int, msg []byte) (*Cosi, []*Cosi) {
	r, children := genPostCommitmentPhaseCosi(nb)
	chal, _ := r.CreateChallenge(msg)
	for _, ch := range children {
		ch.Challenge(chal)
	}
	return r, children
}

func genFinalCosi(nb int, msg []byte) (*Cosi, []*Cosi, error) {
	// go to the challenge phase
	root, children := genPostChallengePhaseCosi(nb, msg)
	var responses []*Response

	// for verification later
	aggResponse := testSuite.Secret().Zero()
	for _, ch := range children {
		// generate the response of each children
		r, err := ch.CreateResponse()
		if err != nil {
			return nil, nil, fmt.Errorf("Error creating response:%v", err)
		}
		responses = append(responses, r)
		aggResponse = aggResponse.Add(aggResponse, r.Response)
	}
	// pass them up to the root
	_, err := root.Response(true, responses)
	if err != nil {
		return nil, nil, fmt.Errorf("Response phase failed:%v", err)
	}
	return root, children, nil
}
