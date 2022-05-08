/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	
	"github.com/spf13/cobra"
)


var nickname string
var tag string

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "search for a single host from a nickname you provide",
	Long: `If you have a "Nickname" tag on your host search using that.

If you don't have the default tag name then you can provide it.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unexpected argument [%s]", args[0] )
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("search called: [%s: %s]\n", tag, nickname)
		
	},
}


func init() {
	rootCmd.AddCommand(searchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// searchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
 	searchCmd.Flags().StringVarP(&nickname, "nickname", "n", "", "Provide the value (or name) to search SSM hosts by tag value. See additional flag for your custom tag key.")
	searchCmd.Flags().StringVarP(&tag, "tag", "t", "Nickname", "Provide the value of a tag name to search SSM hosts by tag value.")

	rootCmd.MarkFlagRequired("nickname")

}
