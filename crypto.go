package zkCrypto

import (
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"math/big"

	"github.com/narula/btcd/btcec"
)

// MAKE SURE TO CALL init() BEFORE DOING ANYTHING
// Global vars used to maintain all the crypto constants
var zkCurve zkpCrypto // look for init()
var H2tothe []ECPoint // look for init()

type side int

const (
	left  side = 0
	right side = 1
)

type ECPoint struct {
	X, Y *big.Int
}

// zkpCrypto is zero knowledge proof curve and params struct, only one instance should be used
type zkpCrypto struct {
	C  elliptic.Curve      // Curve, this is primarily used for it's operations, the Curve itself is not used
	KC *btcec.KoblitzCurve // Curve, this is the Curve used for
	G  ECPoint             // generator 1
	H  ECPoint             // generator 2
	N  *big.Int            // exponent prime
}

// Geeric stuff
func check(e error) {
	if e != nil {
		panic(e)
	}
}

var DEBUG = flag.Bool("debug", false, "Debug output")

// Dprintf is a generic debug statement generator
func Dprintf(format string, args ...interface{}) {
	if *DEBUG {
		fmt.Printf(format, args...)
	}
}

// ============ BASIC ECPoint OPERATIONS ==================

// Equal returns true if points p (self) and p2 (arg) are the same.
func (p ECPoint) Equal(p2 ECPoint) bool {
	if p.X.Cmp(p2.X) == 0 && p2.Y.Cmp(p2.Y) == 0 {
		return true
	}
	return false
}

// Mult multiplies point p by scalar s and returns the resulting point
func (p ECPoint) Mult(s *big.Int) ECPoint {
	modS := new(big.Int).Mod(s, zkCurve.N)
	X, Y := zkCurve.C.ScalarMult(p.X, p.Y, modS.Bytes())
	return ECPoint{X, Y}
}

// Add adds points p and p2 and returns the resulting point
func (p ECPoint) Add(p2 ECPoint) ECPoint {
	X, Y := zkCurve.C.Add(p.X, p.Y, p2.X, p2.Y)
	return ECPoint{X, Y}
}

// Neg returns the addadtive inverse of point p
func (p ECPoint) Neg() ECPoint {
	negY := new(big.Int).Neg(p.Y)
	modValue := negY.Mod(negY, zkCurve.C.Params().P)
	return ECPoint{p.X, modValue}
}

// ============= BASIC zklCrypto OPERATIONS ==================
// These functions are not directly used in the code base much
// TODO: Remove the following functions and just use PedCommits

// CommitR uses the Public Key (pk) and a random number (r mod e.N) to generate a commitment of r as an ECPoint
// A commitment is the locking of a value with a public key that can be posted publically and verifed by everyone
func (e zkpCrypto) CommitR(pk ECPoint, r *big.Int) ECPoint {
	newR := new(big.Int).Mod(r, e.N)                 // newR = r mod e.N to generate a *bigInt
	X, Y := e.C.ScalarMult(pk.X, pk.Y, newR.Bytes()) // {commitR.X,commitR.Y} = newR * {pk.X, pk.Y}
	return ECPoint{X, Y}                             // ECPoint of commited Value
}

// VerifyR checks if the point in question is a valid commitment of R by generating a new point and comparing it
func (e zkpCrypto) VerifyR(rt ECPoint, pk ECPoint, r *big.Int) bool {
	p := e.CommitR(pk, r) // Generate test point (P) using pk and r
	if p.Equal(rt) {
		return true
	}
	return false
}

// Zero generates an ECPoint with the coordinates (0,0) typically to represent inifinty
func (e zkpCrypto) Zero() ECPoint {
	return ECPoint{big.NewInt(0), big.NewInt(0)}
}

// =============== KEYGEN OPERATIONS ==============

// The following code was just copy-pasta'ed into this codebase,
// I trust that the keygen stuff works, if it doesnt ask Willy

func NewECPrimeGroupKey() zkpCrypto {
	curValue := btcec.S256().Gx
	s256 := sha256.New()
	s256.Write(new(big.Int).Add(curValue, big.NewInt(2)).Bytes()) // hash G_x + 2 which

	potentialXValue := make([]byte, 33)
	binary.LittleEndian.PutUint32(potentialXValue, 2)
	for i, elem := range s256.Sum(nil) {
		potentialXValue[i+1] = elem
	}

	gen2, err := btcec.ParsePubKey(potentialXValue, btcec.S256())
	check(err)

	return zkpCrypto{btcec.S256(), btcec.S256(), ECPoint{btcec.S256().Gx,
		btcec.S256().Gy}, ECPoint{gen2.X, gen2.Y}, btcec.S256().N}
}

func KeyGen() (ECPoint, *big.Int) {

	sk, err := rand.Int(rand.Reader, zkCurve.N)
	check(err)
	pkX, pkY := zkCurve.C.ScalarMult(zkCurve.H.X, zkCurve.H.Y, sk.Bytes())

	return ECPoint{pkX, pkY}, sk
}

func DeterministicKeyGen(id int) (ECPoint, *big.Int) {
	idb := big.NewInt(int64(id + 1))
	pkX, pkY := zkCurve.C.ScalarMult(zkCurve.H.X, zkCurve.H.Y, idb.Bytes())
	return ECPoint{pkX, pkY}, idb
}

func GenerateH2tothe() []ECPoint {
	Hslice := make([]ECPoint, 64)
	for i, _ := range Hslice {
		// mv := new(big.Int).Exp(new(big.Int).SetInt64(2), big.NewInt(int64(len(bValue)-i-1)), EC.C.Params().N)
		// This does the same thing.
		m := big.NewInt(1 << uint(i))
		Hslice[i].X, Hslice[i].Y = zkCurve.C.ScalarBaseMult(m.Bytes())
	}
	return Hslice
}

func Init() {
	zkCurve = NewECPrimeGroupKey()
	H2tothe = GenerateH2tothe()
}

// =============== PEDERSEN COMMITMENTS ================

// TODO: figure out if CommitR and PedCommit/R are redundant

// Commit generates a pedersen commitment of (value) using agreeded upon generators of (zkCurve),
// also returns the random value generated for the commitment
func PedCommit(value *big.Int) (ECPoint, *big.Int) {

	// modValue = value mod N
	modValue := new(big.Int).Mod(value, zkCurve.N)

	// randomValue = rand() mod N
	randomValue, err := rand.Int(rand.Reader, zkCurve.N)
	check(err)

	// mG, rH :: lhs, rhs
	lhsX, lhsY := zkCurve.C.ScalarMult(zkCurve.G.X, zkCurve.G.Y, modValue.Bytes())
	rhsX, rhsY := zkCurve.C.ScalarMult(zkCurve.H.X, zkCurve.H.Y, randomValue.Bytes())

	//mG + rH
	commX, commY := zkCurve.C.Add(lhsX, lhsY, rhsX, rhsY)

	return ECPoint{commX, commY}, randomValue
}

// CommitWithR generates a pedersen commitment with a given random value
func PedCommitR(value, randomValue *big.Int) ECPoint {

	// modValue = value mod N
	modValue := new(big.Int).Mod(value, zkCurve.N)

	// randomValue = rand() mod N
	modRandom := new(big.Int).Mod(randomValue, zkCurve.N)

	// mG, rH :: lhs, rhs
	lhsX, lhsY := zkCurve.C.ScalarMult(zkCurve.G.X, zkCurve.G.Y, modValue.Bytes())
	rhsX, rhsY := zkCurve.C.ScalarMult(zkCurve.H.X, zkCurve.H.Y, modRandom.Bytes())

	//mG + rH
	commX, commY := zkCurve.C.Add(lhsX, lhsY, rhsX, rhsY)

	return ECPoint{commX, commY}
}

// Open checks if the values given result in the PedComm being varifed
func Open(value, randomValue *big.Int, PedComm ECPoint) bool {

	// Generate commit using given values
	testCommit := PedCommitR(value, randomValue)
	return testCommit.Equal(PedComm)
}

// =========== GENERALIZED SCHNORR PROOFS ===============

// GSPFS is Generalized Schnorr Proofs with Fiat-Shamir transform
// TODO: change the json stuff

// GSPFSProof is proof of knowledge of x
type GSPFSProof struct {
	RandCommit  ECPoint  `json:"T"` // this is H = uG, where u is random value and G is a generator point
	HiddenValue *big.Int `json:"R"` // s = x * c + u, here c is the challenge and x is what we want to prove knowledge of
	Challenge   *big.Int `json:"C"` // challenge string hash sum, only use for sanity checks
}

/*
	Schnorr Proof: prove that we know x withot revealing x

	Public: generator points G and H

	V									P
	know x								knows A = xG //doesnt know x and G just A
	selects random u
	T1 = uG
	c = HASH(G, xG, uG)
	s = u + c * x

	T1, s, c -------------------------->
										c ?= HASH(G, A, T1)
										sG ?= T1 + cA

*/

// GSPFSProve generates a Schnorr proof for the value x
func GSPFSProve(x *big.Int) *GSPFSProof {

	// res = xG
	resX, resY := zkCurve.C.ScalarMult(zkCurve.G.X, zkCurve.G.Y, x.Bytes())

	// u is a raondom number
	u, err := rand.Int(rand.Reader, zkCurve.N)
	check(err)

	// generate random point uG
	uX, uY := zkCurve.C.ScalarMult(zkCurve.G.X, zkCurve.G.Y, u.Bytes())

	// genereate string to hash for challenge
	stringToHash := zkCurve.G.X.String() + "," + zkCurve.G.Y.String() + "," +
		resX.String() + "," + resY.String() + "," +
		uX.String() + "," + uY.String()

	stringHashed := sha256.Sum256([]byte(stringToHash))

	// c = bigInt(SHA256(stringToHash))
	Challenge := new(big.Int).SetBytes(stringHashed[:])

	// v = u - c * x
	HiddenValue := new(big.Int).Sub(u, new(big.Int).Mul(Challenge, x))
	HiddenValue.Mod(HiddenValue, zkCurve.N)

	return &GSPFSProof{ECPoint{uX, uY}, HiddenValue, Challenge}
}

// TODO: check if result should be within the proof

// Verify checks if a proof-commit pair is valid
func GSPFSVerify(result ECPoint, proof *GSPFSProof) bool {
	// Remeber that result = xG and RandCommit = uG

	hasher := sha256.New()

	stringToHash := zkCurve.G.X.String() + "," + zkCurve.G.Y.String() + "," +
		result.X.String() + "," + result.Y.String() + "," +
		proof.RandCommit.X.String() + "," + proof.RandCommit.Y.String()

	// testC is the challenge string generated from the Proof and commitment being verified
	hasher.Write([]byte(stringToHash))
	testC := new(big.Int).SetBytes(hasher.Sum(nil))

	// (u - c * x)G, look at HiddenValue from GSPFS.Proof()
	sX, sY := zkCurve.C.ScalarMult(zkCurve.G.X, zkCurve.G.Y, proof.HiddenValue.Bytes())

	// cResult = c(xG), we use testC as that follows the proof verficaion process more closely than using Challenge
	cX, cY := zkCurve.C.ScalarMult(result.X, result.Y, testC.Bytes())

	// cxG + (u - cx)G = uG
	totX, totY := zkCurve.C.Add(sX, sY, cX, cY)

	if proof.RandCommit.X.Cmp(totX) != 0 || proof.RandCommit.Y.Cmp(totY) != 0 {
		return false
	}
	return true
}

// =========== EQUIVILANCE PROOFS ===================

type EquivProof struct {
	uG          ECPoint // kG is the scalar mult of k (random num) with base G
	uH          ECPoint
	Challenge   *big.Int // Challenge is hash sum of challenge commitment
	HiddenValue *big.Int // Hidden Value hides the discrete log x that we want to prove equivilance for
}

/*
	Equivilance Proofs: prove that both A and B both use x as a discrete log

	Public: generator points G and H

	V									P
	know x								knows A = xG ; B = xH
	selects random u
	T1 = uG
	T2 = uH
	c = HASH(G, H, xG, xH, uG, uH)
	s = u + c * x

	T1, T2, s, c ---------------------->
										c ?= HASH(G, H, A, B, T1, T2)
										sG ?= T1 + cA
										sH ?= T2 + cB
*/

// EquivilanceProve generates an equivilance proof that Result1 and Result2 use the same discrete log x
func EquivilanceProve(
	Base1, Result1, Base2, Result2 ECPoint, x *big.Int) EquivProof {
	// Base1and Base2 will most likely be G and H, Result1 and Result2 will be xG and xH
	// x trying to be proved that both G and H are raised with x

	checkX, checkY := zkCurve.C.ScalarMult(Base1.X, Base1.Y, x.Bytes())
	if checkX.Cmp(Result1.X) != 0 || checkY.Cmp(Result1.Y) != 0 {
		Dprintf("EquivProof check: Base1 and Result1 are not related by x... \n")
	}
	checkX, checkY = zkCurve.C.ScalarMult(Base2.X, Base2.Y, x.Bytes())
	if checkX.Cmp(Result2.X) != 0 || checkY.Cmp(Result2.Y) != 0 {
		Dprintf("EquivProof check: Base2 and Result2 are not related by x... \n")
	}

	// random number
	u, err := rand.Int(rand.Reader, zkCurve.N) // random number to hide x later
	check(err)

	// uG
	uBase1X, uBase1Y := zkCurve.C.ScalarMult(Base1.X, Base1.Y, u.Bytes())
	// uH
	uBase2X, uBase2Y := zkCurve.C.ScalarMult(Base2.X, Base2.Y, u.Bytes())

	// HASH(G, H, xG, xH, kG, kH)
	stringToHash := Base1.X.String() + "||" + Base1.Y.String() + ";" +
		Base2.X.String() + "||" + Base2.Y.String() + ";" +
		Result1.X.String() + "||" + Result1.Y.String() + ";" +
		Result2.X.String() + "||" + Result2.Y.String() + ";" +
		uBase1X.String() + "||" + uBase1Y.String() + ";" +
		uBase2X.String() + "||" + uBase2Y.String() + ";"

	hasher := sha256.New()
	hasher.Write([]byte(stringToHash))

	Challenge := new(big.Int).SetBytes(hasher.Sum(nil))

	HiddenValue := new(big.Int).Add(u, new(big.Int).Mul(Challenge, x))
	HiddenValue.Mod(HiddenValue, zkCurve.N)

	return EquivProof{
		ECPoint{uBase1X, uBase1Y}, // uG
		ECPoint{uBase2X, uBase2Y}, // uH
		Challenge,
		HiddenValue} //Kinda dumb this bracket cannot be on the next line...

}

// EquivilanceVerify checks if a proof is valid
func EquivilanceVerify(
	Base1, Result1, Base2, Result2 ECPoint, eqProof EquivProof) bool {
	// Regenerate challenge string
	stringToHash := Base1.X.String() + "||" + Base1.Y.String() + ";" +
		Base2.X.String() + "||" + Base2.Y.String() + ";" +
		Result1.X.String() + "||" + Result1.Y.String() + ";" +
		Result2.X.String() + "||" + Result2.Y.String() + ";" +
		eqProof.uG.X.String() + "||" + eqProof.uG.Y.String() + ";" +
		eqProof.uH.X.String() + "||" + eqProof.uH.Y.String() + ";"

	hasher := sha256.New()
	hasher.Write([]byte(stringToHash))

	Challenge := new(big.Int).SetBytes(hasher.Sum(nil))

	if Challenge.Cmp(eqProof.Challenge) != 0 {
		Dprintf(" [crypto] c comparison failed. proof: %v calculated: %v\n",
			eqProof.Challenge, Challenge)
		return false
	}

	// sG ?= uG + cG
	sGX, sGY := zkCurve.C.ScalarMult(Base1.X, Base1.Y, eqProof.HiddenValue.Bytes())
	cGX, cGY := zkCurve.C.ScalarMult(Result1.X, Result1.Y, eqProof.Challenge.Bytes())
	testX, testY := zkCurve.C.Add(eqProof.uG.X, eqProof.uG.Y, cGX, cGY)

	if sGX.Cmp(testX) != 0 || sGY.Cmp(testY) != 0 {
		Dprintf(" [crypto] lhs/rhs cmp failed. lhsX %v lhsY %v rhsX %v rhsY %v\n",
			sGX, sGY, testX, testY)
		return false
	}

	// sH ?= uH + cH
	sHX, sHY := zkCurve.C.ScalarMult(Base2.X, Base2.Y, eqProof.HiddenValue.Bytes())
	cHX, cHY := zkCurve.C.ScalarMult(Result2.X, Result2.Y, eqProof.Challenge.Bytes())
	testX, testY = zkCurve.C.Add(eqProof.uH.X, eqProof.uH.Y, cHX, cHY)

	if sHX.Cmp(testX) != 0 || sHY.Cmp(testY) != 0 {
		Dprintf(" [crypto] lhs/rhs cmp failed. lhsX %v lhsY %v rhsX %v rhsY %v\n",
			sHX, sHY, testX, testY)
		return false
	}

	// All three checks passed, proof must be correct
	return true

}

// The following ia combo of disjunctive proof and equivilance proofs

type EquivORLogProof struct {
	T1 ECPoint  // Either u1 * Base1 or s1*Base1 - c1 * Result1
	T2 ECPoint  // Either u1 * Base2 or s1*Base2 - c1 * Result2
	T3 ECPoint  // Either u2 * Base3 or s2*Base3 - c2 * Result3
	C  *big.Int // Either s1=u1 + c1x or random element
	C1 *big.Int // Either s2=u2 + c2x or random element
	C2 *big.Int // Challenge 1
	S1 *big.Int // Challenge 2
	S2 *big.Int // Sum of challenges
}

/*
	EquivilanceORLog Proofs:
	- Given A = xG, B = xH, D = yJ prove:
		- that A and B both have the same discrete log OR,
		- that we know the discrete log of D

	Public: generator points G, H, and J

	V									P
	Proving A and B use x
	know x AND/OR y						knows A = xG; B = xH; D = yJ // Can all be same base
	selects random u1, u2, u3
	T1 = u1G
	T2 = u1H
	T3 = u3J + (-u2)D // neg(u2)
	c = HASH(G, H, J, A, B, D, T1, T2, T3)
	deltaC = c + (-u2)
	s = u1 + deltaC * x

	T1, T2, T3, c, deltaC, u2, s, u3 -> T1, T2, T3, c, c1, c2, s1, s2
										c ?= HASH(G, H, J, A, B, D, T1, T2, T3)
										s1G ?= T1 + cA
										s1H ?= T2 + cB
										s2J ?= T3 + cD

	===================================================================
	V									P
	To prove that we know y
	know x AND/OR y						knows A = xG; B = xH; D = yJ // Can all be same base
	selects random u1, u2, u3
	T1 = u1G + (-u2)A
	T2 = u1H + (-u2)B
	T3 = u3J
	c = HASH(G, H, J, A, B, D, T1, T2, T3)
	deltaC = c + (-u2)
	s = u1 + deltaC * x

	T1, T2, T3, c, u2, deltaC, u1, s -> T1, T2, T3, c, c1, c2, s1, s2
										c ?= HASH(G, H, J, A, B, D, T1, T2, T3)
										s1G ?= T1 + cA
										s1H ?= T2 + cB
										s2J ?= T3 + cD

*/

func EquivilanceORLogProve(
	Base1, Result1, Base2, Result2, Base3, Result3 ECPoint,
	x *big.Int, option side) EquivORLogProof {

	u1, err := rand.Int(rand.Reader, zkCurve.N)
	check(err)
	u2, err := rand.Int(rand.Reader, zkCurve.N)
	check(err)
	u3, err := rand.Int(rand.Reader, zkCurve.N)
	check(err)

	u2Neg := new(big.Int).Neg(u2)
	u2Neg.Mod(u2Neg, zkCurve.N)

	if option == left { //Proving Equivilance
		// u1G = T1
		T1X, T1Y := zkCurve.C.ScalarMult(Base1.X, Base1.Y, u1.Bytes())
		// u1H = T2
		T2X, T2Y := zkCurve.C.ScalarMult(Base2.X, Base2.Y, u1.Bytes())
		// u3J + (-u2)D = T3
		u3JX, u3JY := zkCurve.C.ScalarMult(Base3.X, Base3.Y, u3.Bytes())
		nu2DX, nu2DY := zkCurve.C.ScalarMult(Result3.X, Result3.Y, u2Neg.Bytes())
		T3X, T3Y := zkCurve.C.Add(u3JX, u3JY, nu2DX, nu2DY)

		// stringToHash = (G, H, J, A, B, D, T1, T2, T3)
		stringToHash := Base1.X.String() + "," + Base1.Y.String() + ";" +
			Base2.X.String() + "," + Base2.Y.String() + ";" +
			Base3.X.String() + "," + Base3.Y.String() + ";" +
			Result1.X.String() + "," + Result1.Y.String() + ";" +
			Result2.X.String() + "," + Result2.Y.String() + ";" +
			Result3.X.String() + "," + Result3.Y.String() + ";" +
			T1X.String() + "," + T1Y.String() + ";" +
			T2X.String() + "," + T2Y.String() + ";" +
			T3X.String() + "," + T3Y.String() + ";"

		hasher := sha256.New()
		hasher.Write([]byte(stringToHash))
		Challenge := new(big.Int).SetBytes(hasher.Sum(nil))
		Challenge = Challenge.Mod(Challenge, zkCurve.N)

		deltaC := new(big.Int).Add(Challenge, u2Neg)
		deltaC.Mod(deltaC, zkCurve.N)

		s := new(big.Int).Add(u1, new(big.Int).Mul(deltaC, x))

		return EquivORLogProof{
			ECPoint{T1X, T1Y},
			ECPoint{T2X, T2Y},
			ECPoint{T3X, T3Y},
			Challenge, deltaC, u2, s, u3}

	} else { // Proving Discrete Log

		// u1G + (-u2A) = T1
		u1GX, u1GY := zkCurve.C.ScalarMult(Base1.X, Base1.Y, u1.Bytes())
		nu2AX, nu2AY := zkCurve.C.ScalarMult(Result1.X, Result1.Y, u2Neg.Bytes())
		T1X, T1Y := zkCurve.C.Add(u1GX, u1GY, nu2AX, nu2AY)
		// u1H + (-u2B) = T2
		u1HX, u1HY := zkCurve.C.ScalarMult(Base2.X, Base2.Y, u1.Bytes())
		nu2BX, nu2BY := zkCurve.C.ScalarMult(Result2.X, Result2.Y, u2Neg.Bytes())
		T2X, T2Y := zkCurve.C.Add(u1HX, u1HY, nu2BX, nu2BY)

		// u3J = T3
		T3X, T3Y := zkCurve.C.ScalarMult(Base3.X, Base3.Y, u3.Bytes())

		// stringToHash = (G, H, J, A, B, D, T1, T2, T3)
		stringToHash := Base1.X.String() + "," + Base1.Y.String() + ";" +
			Base2.X.String() + "," + Base2.Y.String() + ";" +
			Base3.X.String() + "," + Base3.Y.String() + ";" +
			Result1.X.String() + "," + Result1.Y.String() + ";" +
			Result2.X.String() + "," + Result2.Y.String() + ";" +
			Result3.X.String() + "," + Result3.Y.String() + ";" +
			T1X.String() + "," + T1Y.String() + ";" +
			T2X.String() + "," + T2Y.String() + ";" +
			T3X.String() + "," + T3Y.String() + ";"

		hasher := sha256.New()
		hasher.Write([]byte(stringToHash))
		Challenge := new(big.Int).SetBytes(hasher.Sum(nil))
		Challenge = Challenge.Mod(Challenge, zkCurve.N)

		deltaC := new(big.Int).Add(Challenge, u2Neg)
		deltaC.Mod(deltaC, zkCurve.N)

		s := new(big.Int).Add(u1, new(big.Int).Mul(deltaC, x))

		return EquivORLogProof{
			ECPoint{T1X, T1Y},
			ECPoint{T2X, T2Y},
			ECPoint{T3X, T3Y},
			Challenge, u2, deltaC, u3, s}

	}

}

// =============== DISJUNCTIVE PROOFS ========================

// Referance: https://drive.google.com/file/d/0B_ndzgLH0bcvMjg3M1ROUWQwWTBCN0loQ055T212eV9JRU1v/view
// see section 4.2

/*
	Disjunctive Proofs: prove that you know either x or y but do not reveal
						which one you know

	Public: generator points G and H

	V			 						P
	(proving x)
	knows x AND/OR y					knows A = xG ; B = yH // can be yG
	selects random u1, u2, u3
	T1 = u1G
	T2 = u2H + (-u3)yH
	c = HASH(T1, T2, G, A, B)
	deltaC = c - u3
	s = u1 + deltaC * x

	(V perspective)						(P perspective)
	T1, T2, c, deltaC, u3, s, u2 -----> T1, T2, c, c1, c2, s1, s2
										c ?= HASH(T1, T2, G, A, B)
										c ?= c1 + c2 // mod zkCurve.N
										s1G ?= T1 + c1A
										s2G ?= T2 + c2A
	To prove y instead:
	Same as above with y in place of x
	T2, T1, c, u3, deltaC, u2, s -----> T1, T2, c, c1, c2, s1, s2
										Same checks as above

	Note:
	It should be indistingushiable for V with T1, T2, c, c1, c2, s1, s2
	to tell if we are proving x or y. The above arrows show how the variables
	used in the proof translate to T1, T2, etc.

	Sorry about the proof interaction summary above, trying to
	be consice with my comments in this code
*/

// DisjunctiveProof is also Generalized Schnorr Proof with FS-transform
type DisjunctiveProof struct {
	T1 ECPoint
	T2 ECPoint
	C  *big.Int
	C1 *big.Int
	C2 *big.Int
	S1 *big.Int
	S2 *big.Int
}

// DisjunctiveProve generates a disjunctive proof for the given x
func DisjunctiveProve(
	Base1, Result1, Base2, Result2 ECPoint, x *big.Int, option side) *DisjunctiveProof {

	// Declaring them like this because Golang crys otherwise
	ProveBase := zkCurve.Zero()
	ProveResult := zkCurve.Zero()
	OtherBase := zkCurve.Zero()
	OtherResult := zkCurve.Zero()

	// Generate a proof for A
	if option == left {
		ProveBase = Base1
		ProveResult = Result1
		OtherBase = Base2
		OtherResult = Result2
	} else if option == right { // Generate a proof for B
		ProveBase = Base2
		ProveResult = Result2
		OtherBase = Base1
		OtherResult = Result1
	} else { // number for option is not correct
		Dprintf("ERROR --- Invalid option number given for DisjunctiveProve\n")
		return nil
	}

	if !ProveBase.Mult(x).Equal(ProveResult) {
		Dprintf("Seems like we're lying about values we know...", x, ProveBase, ProveResult)
		// TODO: do something with error checking or whatever
		return nil
	}

	u1, err := rand.Int(rand.Reader, zkCurve.N)
	check(err)
	u2, err := rand.Int(rand.Reader, zkCurve.N)
	check(err)
	u3, err := rand.Int(rand.Reader, zkCurve.N)
	check(err)
	// for (-u3)yH
	u3Neg := new(big.Int).Neg(u3)
	u3Neg.Mod(u3Neg, zkCurve.N)

	// T1 = u1G
	T1X, T1Y := zkCurve.C.ScalarMult(ProveBase.X, ProveBase.Y, u1.Bytes())

	// u2H
	tempX, tempY := zkCurve.C.ScalarMult(OtherBase.X, OtherBase.Y, u2.Bytes())
	// (-u3)yH
	temp2X, temp2Y := zkCurve.C.ScalarMult(OtherResult.X, OtherResult.Y, u3Neg.Bytes())
	// T2 = u2H + (-u3)yH (yH is OtherResult)
	T2X, T2Y := zkCurve.C.Add(tempX, tempY, temp2X, temp2Y)

	// String for proving Base1 and Result1
	stringToHash := Base1.X.String() + "," + Base1.Y.String() + ";" +
		Result1.X.String() + "," + Result1.Y.String() + ";" +
		Base2.X.String() + "," + Base2.Y.String() + ";" +
		Result2.X.String() + "," + Result2.Y.String() + ";" +
		T1X.String() + "," + T1Y.String() + ";" +
		T2X.String() + "," + T2Y.String() + ";"

	// If we are proving Base2 and Result2 then we must switch T1 and T2 in string
	if option == 1 {
		stringToHash = Base1.X.String() + "," + Base1.Y.String() + ";" +
			Result1.X.String() + "," + Result1.Y.String() + ";" +
			Base2.X.String() + "," + Base2.Y.String() + ";" +
			Result2.X.String() + "," + Result2.Y.String() + ";" +
			T2X.String() + "," + T2Y.String() + ";" +
			T1X.String() + "," + T1Y.String() + ";"
	}

	hasher := sha256.New()
	hasher.Write([]byte(stringToHash))
	Challenge := new(big.Int).SetBytes(hasher.Sum(nil))

	deltaC := new(big.Int).Sub(Challenge, u3)
	deltaC.Mod(deltaC, zkCurve.N)

	s := new(big.Int).Add(u1, new(big.Int).Mul(deltaC, x))

	// Look at mapping given in block comment above
	if option == left {
		return &DisjunctiveProof{
			ECPoint{T1X, T1Y},
			ECPoint{T2X, T2Y},
			Challenge,
			deltaC,
			u3,
			s,
			u2}
	} else {
		return &DisjunctiveProof{
			ECPoint{T2X, T2Y},
			ECPoint{T1X, T1Y},
			Challenge,
			u3,
			deltaC,
			u2,
			s}
	}

	// // Should never reach this statement, best not to have undefined behaviour though
	// Dprintf("ERROR --- Should not be here loc: AAA")
	// return nil
}

/*
	Copy-Pasta from above for convienence
	GIVEN: T1, T2, c, c1, c2, s1, s2
	c ?= HASH(T1, T2, G, A, B)
	c ?= c1 + c2 // mod zkCurve.N
	s1G ?= T1 + c1A
	s2G ?= T2 + c2A
*/

// DisjunctiveVerify checks if a djProof is valid for the given bases and results
func DisjunctiveVerify(
	Base1, Result1, Base2, Result2 ECPoint, djProof *DisjunctiveProof) bool {

	T1 := djProof.T1
	T2 := djProof.T2
	C := djProof.C
	C1 := djProof.C1
	C2 := djProof.C2
	S1 := djProof.S1
	S2 := djProof.S2

	stringToHash := Base1.X.String() + "," + Base1.Y.String() + ";" +
		Result1.X.String() + "," + Result1.Y.String() + ";" +
		Base2.X.String() + "," + Base2.Y.String() + ";" +
		Result2.X.String() + "," + Result2.Y.String() + ";" +
		T1.X.String() + "," + T1.Y.String() + ";" +
		T2.X.String() + "," + T2.Y.String() + ";"

	hasher := sha256.New()
	hasher.Write([]byte(stringToHash))
	// C
	checkC := new(big.Int).SetBytes(hasher.Sum(nil))
	if checkC.Cmp(C) != 0 {
		Dprintf("DJproof failed : checkC does not agree with proofC\n")
		return false
	}

	// C1 + C2
	totalC := new(big.Int).Add(C1, C2)
	totalC.Mod(totalC, zkCurve.N)
	if totalC.Cmp(C) != 0 {
		Dprintf("DJproof failed : totalC does not agree with proofC\n")
		return false
	}

	// T1 + c1A
	c1AX, c1AY := zkCurve.C.ScalarMult(Result1.X, Result1.Y, C1.Bytes())
	checks1GX, checks1GY := zkCurve.C.Add(c1AX, c1AY, T1.X, T1.Y)
	s1GX, s1GY := zkCurve.C.ScalarMult(Base1.X, Base1.Y, S1.Bytes())

	if checks1GX.Cmp(s1GX) != 0 || checks1GY.Cmp(s1GY) != 0 {
		Dprintf("DJproof failed : s1G not equal to T1 + c1A\n")
		return false
	}

	// T2 + c2B
	c2AX, c2AY := zkCurve.C.ScalarMult(Result2.X, Result2.Y, C2.Bytes())
	checks2GX, checks2GY := zkCurve.C.Add(c2AX, c2AY, T2.X, T2.Y)
	s2GX, s2GY := zkCurve.C.ScalarMult(Base2.X, Base2.Y, S2.Bytes())

	if checks2GX.Cmp(s2GX) != 0 || checks2GY.Cmp(s2GY) != 0 {
		Dprintf("DJproof failed : s2G not equal to T2 + c2B\n")
		return false
	}

	return true
}

// ============ zkLedger Stuff =======================
// ============ Consistance Proofs ===================

type ConsistencyProof struct {
	A1 ECPoint
	A2 ECPoint
	C  *big.Int
	R1 *big.Int
	R2 *big.Int
}
