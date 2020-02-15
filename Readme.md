# GoFakeJson

Generate data for fake test json, using an exhaustive permutation of the input data

The scope of this tool is to generate some fake json data by a given data, using permutation.

The tool take three parameters:

- `-json`: path of the json file to take as template
- `-conf`: path of the data that have to be generated
- `-output`: folder created that will contains the data

The `-json` parameter have to be the path of the json.
The `-conf` file is an `.ini` file that have to respect the following rules:

- Nested data are separated with `dot` (.)
- Array index are expressed as `[index]`
- The data that have to be inserted have to be after the `=`, splitted by `,`

## Example

Assume that we have the following JSON and we want to create some data changing the type of `eventName`

```json
{
  "Records": [
    {
      "eventID": "f07f8ca4b0b26cb9c4e5e77e69f274ee",
      "eventName": "INSERT",
      "eventVersion": "1.1",
      "eventSource": "aws:dynamodb",
      "awsRegion": "us-east-1",
      "userIdentity": {
        "type": "Service",
        "principalId": "dynamodb.amazonaws.com"
      },
      "dynamodb": {
        "ApproximateCreationDateTime": 1480642020,
        "Keys": {
          "val": {
            "S": "data"
          },
          "key": {
            "S": "binary"
          }
        },
        "NewImage": {
          "val": {
            "S": "data"
          },
          "asdf1": {
            "B": "AAEqQQ=="
          },
          "asdf2": {
            "BS": [
              "AAEqQQ==",
              "QSoBAA=="
            ]
          },
          "key": {
            "S": "binary"
          }
        },
        "SequenceNumber": "1405400000000002063282832",
        "SizeBytes": 54,
        "StreamViewType": "NEW_AND_OLD_IMAGES"
      },
      "eventSourceARN": "arn:aws:dynamodb:us-east-1:123456789012:table/Example-Table/stream/2016-12-01T00:00:00.000"
    }
  ]
}
```

The configuration file will be as following:

```ini
Records.[0].eventName=INSERT,REMOVE,MODIFY
```

If we need to generate more data using different combination:

```ini
Records.[0].eventName=INSERT,REMOVE,MODIFY
Records.[0].dynamodb.NewImage.val.S=data,test,prova
```
