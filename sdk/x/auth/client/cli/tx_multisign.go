package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/tendermint/classic/crypto/multisig"
	"github.com/tendermint/classic/libs/cli"
	"github.com/tendermint/go-amino-x"

	"github.com/tendermint/classic/sdk/client/context"
	"github.com/tendermint/classic/sdk/client/flags"
	"github.com/tendermint/classic/sdk/client/keys"
	crkeys "github.com/tendermint/classic/sdk/crypto/keys"
	"github.com/tendermint/classic/sdk/version"
	"github.com/tendermint/classic/sdk/x/auth/client/utils"
	"github.com/tendermint/classic/sdk/x/auth/types"
)

// GetSignCommand returns the sign command
func GetMultiSignCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multisign [file] [name] [[signature]...]",
		Short: "Generate multisig signatures for transactions generated offline",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Sign transactions created with the --generate-only flag that require multisig signatures.

Read signature(s) from [signature] file(s), generate a multisig signature compliant to the
multisig key [name], and attach it to the transaction read from [file].

Example:
$ %s multisign transaction.json k1k2k3 k1sig.json k2sig.json k3sig.json

If the flag --signature-only flag is on, it outputs a JSON representation
of the generated signature only.

The --offline flag makes sure that the client will not reach out to an external node.
Thus account number or sequence number lookups will not be performed and it is
recommended to set such parameters manually.
`,
				version.ClientName,
			),
		),
		RunE: makeMultiSignCmd(),
		Args: cobra.MinimumNArgs(3),
	}

	cmd.Flags().Bool(flagSigOnly, false, "Print only the generated signature, then exit")
	cmd.Flags().Bool(flagOffline, false, "Offline mode. Do not query a full node")
	cmd.Flags().String(flagOutfile, "", "The document will be written to the given file instead of STDOUT")

	// Add the flags here and return the command
	return flags.PostCommands(cmd)[0]
}

func makeMultiSignCmd() func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) (err error) {
		stdTx, err := utils.ReadStdTxFromFile(args[0])
		if err != nil {
			return
		}

		keybase, err := keys.NewKeyBaseFromDir(viper.GetString(cli.HomeFlag))
		if err != nil {
			return
		}

		multisigInfo, err := keybase.Get(args[1])
		if err != nil {
			return
		}
		if multisigInfo.GetType() != crkeys.TypeMulti {
			return fmt.Errorf("%q must be of type %s: %s", args[1], crkeys.TypeMulti, multisigInfo.GetType())
		}

		multisigPub := multisigInfo.GetPubKey().(multisig.PubKeyMultisigThreshold)
		multisigSig := multisig.NewMultisig(len(multisigPub.PubKeys))
		cliCtx := context.NewCLIContext()
		txBldr := types.NewTxBuilderFromCLI()

		if !viper.GetBool(flagOffline) {
			accnum, seq, err := types.NewAccountRetriever(cliCtx).GetAccountNumberSequence(multisigInfo.GetAddress())
			if err != nil {
				return err
			}

			txBldr = txBldr.WithAccountNumber(accnum).WithSequence(seq)
		}

		// read each signature and add it to the multisig if valid
		for i := 2; i < len(args); i++ {
			stdSig, err := readAndUnmarshalStdSignature(args[i])
			if err != nil {
				return err
			}

			// Validate each signature
			sigBytes := types.StdSignBytes(
				txBldr.ChainID(), txBldr.AccountNumber(), txBldr.Sequence(),
				stdTx.Fee, stdTx.GetMsgs(), stdTx.GetMemo(),
			)
			if ok := stdSig.PubKey.VerifyBytes(sigBytes, stdSig.Signature); !ok {
				return fmt.Errorf("couldn't verify signature")
			}
			if err := multisigSig.AddSignatureFromPubKey(stdSig.Signature, stdSig.PubKey, multisigPub.PubKeys); err != nil {
				return err
			}
		}

		newStdSig := types.StdSignature{Signature: amino.MustMarshal(multisigSig), PubKey: multisigPub}
		newTx := types.NewStdTx(stdTx.GetMsgs(), stdTx.Fee, []types.StdSignature{newStdSig}, stdTx.GetMemo())

		sigOnly := viper.GetBool(flagSigOnly)
		var json []byte
		switch {
		case sigOnly && cliCtx.Indent:
			json, err = amino.MarshalJSONIndent(newTx.Signatures[0], "", "  ")
		case sigOnly && !cliCtx.Indent:
			json, err = amino.MarshalJSON(newTx.Signatures[0])
		case !sigOnly && cliCtx.Indent:
			json, err = amino.MarshalJSONIndent(newTx, "", "  ")
		default:
			json, err = amino.MarshalJSON(newTx)
		}
		if err != nil {
			return err
		}

		if viper.GetString(flagOutfile) == "" {
			fmt.Printf("%s\n", json)
			return
		}

		fp, err := os.OpenFile(
			viper.GetString(flagOutfile), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644,
		)
		if err != nil {
			return err
		}
		defer fp.Close()

		fmt.Fprintf(fp, "%s\n", json)

		return
	}
}

func readAndUnmarshalStdSignature(filename string) (stdSig types.StdSignature, err error) {
	var bytes []byte
	if bytes, err = ioutil.ReadFile(filename); err != nil {
		return
	}
	if err = amino.UnmarshalJSON(bytes, &stdSig); err != nil {
		return
	}
	return
}
