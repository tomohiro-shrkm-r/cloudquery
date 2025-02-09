package dynamodb

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/cloudquery/cloudquery/plugins/source/aws/client"
	"github.com/cloudquery/plugin-sdk/v2/schema"
	"github.com/cloudquery/plugin-sdk/v2/transformers"
)

func Tables() *schema.Table {
	tableName := "aws_dynamodb_tables"
	return &schema.Table{
		Name:                tableName,
		Description:         `https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TableDescription.html`,
		Resolver:            fetchDynamodbTables,
		PreResourceResolver: getTable,
		Multiplex:           client.ServiceAccountRegionMultiplexer(tableName, "dynamodb"),
		Transform:           transformers.TransformWithStruct(&types.TableDescription{}),
		Columns: []schema.Column{
			client.DefaultAccountIDColumn(false),
			client.DefaultRegionColumn(false),
			{
				Name:     "tags",
				Type:     schema.TypeJSON,
				Resolver: resolveDynamodbTableTags,
			},
			{
				Name:     "arn",
				Type:     schema.TypeString,
				Resolver: schema.PathResolver("TableArn"),
				CreationOptions: schema.ColumnCreationOptions{
					PrimaryKey: true,
				},
			},
			{
				Name:     "archival_summary",
				Type:     schema.TypeJSON,
				Resolver: schema.PathResolver("ArchivalSummary"),
			},
		},
		Relations: []*schema.Table{
			tableReplicaAutoScalings(),
			tableContinuousBackups(),
		},
	}
}

func fetchDynamodbTables(ctx context.Context, meta schema.ClientMeta, parent *schema.Resource, res chan<- any) error {
	c := meta.(*client.Client)
	svc := c.Services().Dynamodb

	config := dynamodb.ListTablesInput{}
	// No paginator available
	for {
		output, err := svc.ListTables(ctx, &config, func(options *dynamodb.Options) {
			options.Region = c.Region
		})
		if err != nil {
			return err
		}
		res <- output.TableNames

		if aws.ToString(output.LastEvaluatedTableName) == "" {
			break
		}
		config.ExclusiveStartTableName = output.LastEvaluatedTableName
	}

	return nil
}

func getTable(ctx context.Context, meta schema.ClientMeta, resource *schema.Resource) error {
	c := meta.(*client.Client)
	svc := c.Services().Dynamodb

	tableName := resource.Item.(string)

	response, err := svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &tableName}, func(options *dynamodb.Options) {
		options.Region = c.Region
	})
	if err != nil {
		return err
	}

	resource.Item = response.Table
	return nil
}

func resolveDynamodbTableTags(ctx context.Context, meta schema.ClientMeta, resource *schema.Resource, c schema.Column) error {
	table := resource.Item.(*types.TableDescription)

	cl := meta.(*client.Client)
	svc := cl.Services().Dynamodb
	var tags []types.Tag
	input := &dynamodb.ListTagsOfResourceInput{
		ResourceArn: table.TableArn,
	}
	// No paginator available
	for {
		res, err := svc.ListTagsOfResource(ctx, input, func(options *dynamodb.Options) {
			options.Region = cl.Region
		})
		if err != nil {
			if cl.IsNotFoundError(err) {
				return nil
			}
			return err
		}
		tags = append(tags, res.Tags...)
		if aws.ToString(res.NextToken) == "" {
			break
		}
		input.NextToken = res.NextToken
	}
	return resource.Set(c.Name, client.TagsToMap(tags))
}

func fetchDynamodbTableReplicaAutoScalings(ctx context.Context, meta schema.ClientMeta, parent *schema.Resource, res chan<- any) error {
	par := parent.Item.(*types.TableDescription)

	if aws.ToString(par.GlobalTableVersion) == "" {
		// "This operation only applies to Version 2019.11.21 of global tables"
		return nil
	}

	c := meta.(*client.Client)
	svc := c.Services().Dynamodb

	output, err := svc.DescribeTableReplicaAutoScaling(ctx, &dynamodb.DescribeTableReplicaAutoScalingInput{
		TableName: par.TableName,
	}, func(options *dynamodb.Options) {
		options.Region = c.Region
	})
	if err != nil {
		if c.IsNotFoundError(err) {
			return nil
		}
		return err
	}

	for i := range output.TableAutoScalingDescription.Replicas {
		res <- output.TableAutoScalingDescription.Replicas[i]
	}
	return nil
}
func fetchDynamodbTableContinuousBackups(ctx context.Context, meta schema.ClientMeta, parent *schema.Resource, res chan<- any) error {
	par := parent.Item.(*types.TableDescription)

	c := meta.(*client.Client)
	svc := c.Services().Dynamodb

	output, err := svc.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{
		TableName: par.TableName,
	}, func(options *dynamodb.Options) {
		options.Region = c.Region
	})
	if err != nil {
		if c.IsNotFoundError(err) {
			return nil
		}
		return err
	}

	res <- output.ContinuousBackupsDescription
	return nil
}
