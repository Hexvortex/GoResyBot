/*
Author: Bruce Jagid
Created On: Aug 12, 2023
*/
package resy 

import (
    "github.com/21Bruce/resolved-server/api"
    "net/http"
    "net/url"
    "encoding/json"
    "io"
    "bytes"
    "strconv"
    "strings"
    "time"
    "fmt"
)

/*
Name: API
Type: API interface struct
Purpose: This struct acts as the resy implementation of the 
api interface. 
Note: The only known working APIKey value can be located and
defaulted using the GetDefaultAPI function, but we leave
it exposed so front-facing wrappers may expose it as a
setting
*/
type API struct {
    APIKey      string 
}

/*
Name: isCodeFail 
Type: Internal Func 
Purpose: Function which takes in an HTTP code and returns
true if it is not a success code and false otherwise
*/
func isCodeFail(code int) (bool) {
    fst := code / 100
    return (fst != 2)  
}

/*
Name: byteToJSONString 
Type: Internal Func 
Purpose: Function which takes in a byte sequence 
representing a JSON struct and returns a string 
or error. Useful for debugging
*/
func byteToJSONString(data []byte) (string, error) {
    var out bytes.Buffer
    err := json.Indent(&out, data, "", " ")

    if err != nil {
        return "", err
    }

    d := out.Bytes()
    return string(d), nil
}

/*
Name: min 
Type: Internal Func 
Purpose: Function that determins the min of two ints
*/
func min(a,b int) (int) {
    if a < b {
        return a
    }
    return b
}

/*
Name: GetDefaultAPI 
Type: External Func 
Purpose: Function that provides an out of the box
working API struct
*/
func GetDefaultAPI() (API){
    return API{
        APIKey: "VbWk7s3L4KiK5fzlO7JD3Q5EYolJI7n5",
    }
}

/*
Name: Login 
Type: API Func 
Purpose: Resy implementation of the Login api func
Note: The only required login fields for this func 
are Email and Password.
*/
func (a *API) Login(params api.LoginParam) (*api.LoginResponse, error) {
    authUrl := "https://api.resy.com/3/auth/password"
    email := url.QueryEscape(params.Email)
    password := url.QueryEscape(params.Password)
    bodyStr :=`email=` + email + `&password=` + password
    bodyBytes := []byte(bodyStr)

    request, err := http.NewRequest("POST", authUrl, bytes.NewBuffer(bodyBytes))
    if err != nil {
        return nil, err
    }
    
    request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    request.Header.Set("Authorization", `ResyAPI api_key="` + a.APIKey + `"`)

    client := &http.Client{}
    response, err := client.Do(request)

    if err != nil {
        return nil, err
    }

    // Resy servers return a 419 is the auth parameters were invalid
    if response.StatusCode == 419 {
        return nil, api.ErrLoginWrong
    }

    if isCodeFail(response.StatusCode) {
        return nil, api.ErrNetwork
    }

    defer response.Body.Close()

    responseBody, err := io.ReadAll(response.Body)

    if err != nil {
        return nil, err
    }


    var jsonMap map[string]interface{}
    err = json.Unmarshal(responseBody, &jsonMap)
    if err != nil {
        return nil, err
    }

    if jsonMap["payment_method_id"] == nil {
        return nil, api.ErrNoPayInfo
    }


    loginResponse := api.LoginResponse{
        ID:              int64(jsonMap["id"].(float64)),
        FirstName:       jsonMap["first_name"].(string),
        LastName:        jsonMap["last_name"].(string),
        Mobile:          jsonMap["mobile_number"].(string),
        Email:           jsonMap["em_address"].(string),
        PaymentMethodID: int64(jsonMap["payment_method_id"].(float64)),
        AuthToken:       jsonMap["token"].(string),
    }

    return &loginResponse, nil

}

/*
Name: Search 
Type: API Func 
Purpose: Resy implementation of the Search api func
*/
func (a *API) Search(params api.SearchParam) (*api.SearchResponse, error) {
    searchUrl := "https://api.resy.com/3/venuesearch/search"
    fmt.Print("s3earch url",searchUrl)

    bodyStr :=`{"query":"` + params.Name +`"}`
    bodyBytes := []byte(bodyStr)

    request, err := http.NewRequest("POST", searchUrl, bytes.NewBuffer(bodyBytes))
    if err != nil {
        return nil, err
    }
    
    request.Header.Set("Content-Type", "application/json")
    request.Header.Set("Authorization", `ResyAPI api_key="` + a.APIKey + `"`)
    request.Header.Set("Origin", `https://resy.com`)
    request.Header.Set("Referer", `https://resy.com/`)

    client := &http.Client{}
    response, err := client.Do(request)

    if err != nil {
        return nil, err
    }

    if isCodeFail(response.StatusCode) {
        return nil, api.ErrNetwork
    }

    defer response.Body.Close()

    responseBody, err := io.ReadAll(response.Body)
    if err != nil {
        return nil, err
    }
    
    var jsonTopLevelMap map[string]interface{}
    err = json.Unmarshal(responseBody, &jsonTopLevelMap)
    if err != nil {
        return nil, err
    }

    jsonSearchMap := jsonTopLevelMap["search"].(map[string]interface{})

    jsonHitsMap := jsonSearchMap["hits"].([]interface{}) 
    numHits := len(jsonHitsMap)

    // if input param limit is nonnegative, limit the search loop
    var limit int 
    if params.Limit > 0 {
        limit = min(params.Limit, numHits)
    } else {
        limit = numHits
    }
    searchResults := make([]api.SearchResult, limit, limit)
    for i:=0; i<limit; i++ {
        jsonHitMap := jsonHitsMap[i].(map[string]interface{})
        venueID, err := strconv.ParseInt(jsonHitMap["objectID"].(string), 10, 64)
        if err != nil {
            return nil, err
        }
        searchResults[i] = api.SearchResult{
            VenueID:      venueID,
            Name:         jsonHitMap["name"].(string), 
            Region:       jsonHitMap["region"].(string), 
            Locality:     jsonHitMap["locality"].(string), 
            Neighborhood: jsonHitMap["neighborhood"].(string), 
        }
    }

    searchResponse := api.SearchResponse{
        Results: searchResults,
    }

    return &searchResponse, nil
}

/*
Name: Reserve
Type: API Func 
Purpose: Resy implementation of the Reserve api func
*/
func (a *API) Reserve(params api.ReserveParam) (*api.ReserveResponse, error) {
    fmt.Println("Starting Reserve function")
    defer fmt.Println("Exiting Reserve function")

    // Converting fields to URL query format
    fmt.Println("Converting reservation times to date string")
    year := strconv.Itoa(params.ReservationTimes[0].Year())
    month := strconv.Itoa(int(params.ReservationTimes[0].Month()))
    day := strconv.Itoa(params.ReservationTimes[0].Day())
    date := year + "-" + month + "-" + day
    fmt.Printf("Formatted date: %s\n", date)

    dayField := `day=` + date
    authField := `x-resy-auth-token=` + params.LoginResp.AuthToken
    latField := `lat=0`
    longField := `long=0`
    venueIDField := `venue_id=` + strconv.FormatInt(params.VenueID, 10)
    partySizeField := `party_size=` + strconv.Itoa(params.PartySize)
    fields := []string{dayField, authField, latField, longField, venueIDField, partySizeField}

    findUrl := `https://api.resy.com/4/find?` + strings.Join(fields, "&")
    fmt.Printf("Find URL: %s\n", findUrl)

    request, err := http.NewRequest("GET", findUrl, bytes.NewBuffer([]byte{}))
    if err != nil {
        fmt.Printf("Error creating find request: %v\n", err)
        return nil, err
    }

    // Setting headers
    fmt.Println("Setting headers for find request")
    request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    request.Header.Set("Authorization", `ResyAPI api_key="`+a.APIKey+`"`)
    request.Header.Set("X-Resy-Auth-Token", params.LoginResp.AuthToken)
    request.Header.Set("X-Resy-Universal-Auth", params.LoginResp.AuthToken)
    request.Header.Set("Referer", "https://resy.com/")

    client := &http.Client{}
    fmt.Println("Sending find request")
    response, err := client.Do(request)
    if err != nil {
        fmt.Printf("Error sending find request: %v\n", err)
        return nil, err
    }
    fmt.Printf("Received find response with status code: %d\n", response.StatusCode)

    if isCodeFail(response.StatusCode) {
        fmt.Printf("Find request failed with status code: %d\n", response.StatusCode)
        return nil, api.ErrNetwork
    }

    defer response.Body.Close()

    responseBody, err := io.ReadAll(response.Body)
    if err != nil {
        fmt.Printf("Error reading find response body: %v\n", err)
        return nil, err
    }
    fmt.Printf("Find response body: %s\n", string(responseBody))

    var jsonTopLevelMap map[string]interface{}
    err = json.Unmarshal(responseBody, &jsonTopLevelMap)
    if err != nil {
        fmt.Printf("Error unmarshaling find response JSON: %v\n", err)
        return nil, err
    }

    // Navigate JSON structure
    fmt.Println("Parsing JSON response for venues and slots")
    jsonResultsMap, ok := jsonTopLevelMap["results"].(map[string]interface{})
    if !ok {
        fmt.Println("Error: 'results' key not found or invalid in JSON response")
        return nil, api.ErrNetwork
    }

    jsonVenuesList, ok := jsonResultsMap["venues"].([]interface{})
    if !ok {
        fmt.Println("Error: 'venues' key not found or invalid in JSON response")
        return nil, api.ErrNetwork
    }

    if len(jsonVenuesList) == 0 {
        fmt.Println("No venues found in the response")
        return nil, api.ErrNoOffer
    }

    jsonVenueMap, ok := jsonVenuesList[0].(map[string]interface{})
    if !ok {
        fmt.Println("Error: Invalid venue structure in JSON response")
        return nil, api.ErrNetwork
    }

    jsonSlotsList, ok := jsonVenueMap["slots"].([]interface{})
    if !ok {
        fmt.Println("Error: 'slots' key not found or invalid in venue JSON")
        return nil, api.ErrNetwork
    }

    fmt.Printf("Number of slots available: %d\n", len(jsonSlotsList))

    // Iterate over table types and reservation times
    for k := 0; k < len(params.TableTypes) || (len(params.TableTypes) == 0 && k == 0); k++ {
        var currentTableType api.TableType
        if len(params.TableTypes) != 0 {
            currentTableType = params.TableTypes[k]
            fmt.Printf("Searching for table type: %s\n", currentTableType)
        } else {
            currentTableType = api.DiningRoom
            fmt.Printf("No specific table type provided. Using default: %s\n", currentTableType)
        }

        for i := 0; i < len(params.ReservationTimes); i++ {
            currentTime := params.ReservationTimes[i]
            fmt.Printf("Checking reservation time: %s\n", currentTime.Format("2006-01-02 15:04:00"))

            for j := 0; j < len(jsonSlotsList); j++ {
                fmt.Printf("Evaluating slot %d\n", j)
                jsonSlotMap, ok := jsonSlotsList[j].(map[string]interface{})
                if !ok {
                    fmt.Printf("Error: Invalid slot structure at index %d\n", j)
                    continue
                }

                jsonDateMap, ok := jsonSlotMap["date"].(map[string]interface{})
                if !ok {
                    fmt.Printf("Error: 'date' key missing or invalid in slot %d\n", j)
                    continue
                }

                startRaw, ok := jsonDateMap["start"].(string)
                if !ok {
                    fmt.Printf("Error: 'start' key missing or invalid in slot %d\n", j)
                    continue
                }
                fmt.Printf("Slot start time: %s\n", startRaw)

                startFields := strings.Split(startRaw, " ")
                if len(startFields) != 2 {
                    fmt.Printf("Error: Unexpected 'start' format in slot %d\n", j)
                    continue
                }

                timeFields := strings.Split(startFields[1], ":")
                if len(timeFields) != 3 {
                    fmt.Printf("Error: Unexpected time format in slot %d\n", j)
                    continue
                }

                hourFieldInt, err := strconv.Atoi(timeFields[0])
                if err != nil {
                    fmt.Printf("Error parsing hour in slot %d: %v\n", j, err)
                    continue
                }

                minFieldInt, err := strconv.Atoi(timeFields[1])
                if err != nil {
                    fmt.Printf("Error parsing minute in slot %d: %v\n", j, err)
                    continue
                }

                jsonConfigMap, ok := jsonSlotMap["config"].(map[string]interface{})
                if !ok {
                    fmt.Printf("Error: 'config' key missing or invalid in slot %d\n", j)
                    continue
                }

                tableType, ok := jsonConfigMap["type"].(string)
                if !ok {
                    fmt.Printf("Error: 'type' key missing or invalid in config of slot %d\n", j)
                    continue
                }
                fmt.Printf("Slot table type: %s\n", tableType)

                // Check if the slot matches the desired time and table type
                if hourFieldInt == currentTime.Hour() && minFieldInt == currentTime.Minute() &&
                    (len(params.TableTypes) == 0 || strings.Contains(strings.ToLower(tableType), string(currentTableType))) {
                    fmt.Printf("Found matching slot at index %d for time %s and table type %s\n", j, currentTime.Format("15:04"), currentTableType)

                    configToken, ok := jsonConfigMap["token"].(string)
                    if !ok {
                        fmt.Printf("Error: 'token' key missing or invalid in config of slot %d\n", j)
                        continue
                    }

                    detailUrl := "https://api.resy.com/3/details"
                    fmt.Printf("Detail URL: %s\n", detailUrl)

                    // Prepare the request body
                    requestBody := map[string]string{
                        "commit":     strconv.Itoa(1),                  // Convert integer 1 to string
                        "config_id":  configToken,                      // Assuming configToken is already a string
                        "day":        date,                             // Assuming date is already a string
                        "party_size": strconv.Itoa(params.PartySize),   // Convert PartySize (an int) to string
                    }
                    jsonBody, err := json.Marshal(requestBody)
                     
                    if err != nil {
                        fmt.Printf("Error marshaling request body: %v\n", err)
                        continue
                    }
                    fmt.Printf("Request Body: %s\n", string(jsonBody)) // Add this line

                    requestDetail, err := http.NewRequest("POST", detailUrl, bytes.NewBuffer(jsonBody))
                    if err != nil {
                        fmt.Printf("Error creating detail request: %v\n", err)
                        continue
                    }

                    // Setting headers for detail request
                    // Set the appropriate headers
                    requestDetail.Header.Set("Content-Type", "application/json")
                    requestDetail.Header.Set("Authorization", "ResyAPI api_key=\"VbWk7s3L4KiK5fzlO7JD3Q5EYolJI7n5\"")
                    requestDetail.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
                    // Log the request headers
                    fmt.Println("Request Headers:")
                    for key, value := range requestDetail.Header {
                        fmt.Printf("%s: %s\n", key, strings.Join(value, ", "))
                    }

                    fmt.Println("Sending detail request")
                    responseDetail, err := client.Do(requestDetail)
                    print(responseDetail)
                    if err != nil {
                        fmt.Printf("Error sending detail request: %v\n", err)
                        continue
                    }
                    fmt.Printf("Received detail response with status code: %d\n", responseDetail.StatusCode)

                    if isCodeFail(responseDetail.StatusCode) {
                        responseDetailBody, err := io.ReadAll(responseDetail.Body)
                        if err != nil {
                            fmt.Printf("Error reading detail response body: %v\n", err)
                            continue
                        }
                        fmt.Printf("Detail response body: %s\n", string(responseDetailBody))
                        fmt.Printf("Detail request failed with status code: %d\n", responseDetail.StatusCode)
                        return nil, api.ErrNetwork
                    }

                    defer responseDetail.Body.Close()

                    responseDetailBody, err := io.ReadAll(responseDetail.Body)
                    fmt.Printf("Detail response body: %s\n", string(responseDetailBody))
                    if err != nil {
                        fmt.Printf("Error reading detail response body: %v\n", err)
                        continue
                    }
                    fmt.Printf("Detail response body: %s\n", string(responseDetailBody))

                    var detailTopLevelMap map[string]interface{}
                    err = json.Unmarshal(responseDetailBody, &detailTopLevelMap)
                    if err != nil {
                        fmt.Printf("Error unmarshaling detail response JSON: %v\n", err)
                        return nil, err
                    }

                    jsonBookTokenMap, ok := detailTopLevelMap["book_token"].(map[string]interface{})
                    if !ok {
                        fmt.Println("Error: 'book_token' key missing or invalid in detail JSON")
                        continue
                    }

                    bookToken, ok := jsonBookTokenMap["value"].(string)
                    if !ok {
                        fmt.Println("Error: 'value' key missing or invalid in 'book_token'")
                        continue
                    }
                    fmt.Printf("Obtained book token: %s\n", bookToken)

                    // Proceed to booking step
                    bookUrl := "https://api.resy.com/3/book"
                    fmt.Printf("Book URL: %s\n", bookUrl)

                    bookField := "book_token=" + url.QueryEscape(bookToken)
                    paymentMethodStr := `{"id":` + strconv.FormatInt(params.LoginResp.PaymentMethodID, 10) + `}`
                    paymentMethodField := "struct_payment_method=" + url.QueryEscape(paymentMethodStr)
                    requestBookBodyStr := bookField + "&" + paymentMethodField + "&" + "source_id=resy.com-venue-details"
                    fmt.Printf("Book request body: %s\n", requestBookBodyStr)

                    requestBook, err := http.NewRequest("POST", bookUrl, bytes.NewBuffer([]byte(requestBookBodyStr)))
                    if err != nil {
                        fmt.Printf("Error creating book request: %v\n", err)
                        continue
                    }

                    // Setting headers for book request
                    fmt.Println("Setting headers for book request")
                    requestBook.Header.Set("Authorization", `ResyAPI api_key="`+a.APIKey+`"`)
                    requestBook.Header.Set("Content-Type", `application/x-www-form-urlencoded`)
                    requestBook.Header.Set("Host", `api.resy.com`)
                    requestBook.Header.Set("X-Resy-Auth-Token", params.LoginResp.AuthToken)
                    requestBook.Header.Set("X-Resy-Universal-Auth", params.LoginResp.AuthToken)
                    requestBook.Header.Set("Referer", "https://resy.com/")
                    requestBook.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

                    fmt.Println("Sending book request")  
                    responseBook, err := client.Do(requestBook)
                    if err != nil {
                        fmt.Printf("Error sending book request: %v\n", err)
                        continue
                    }
                    fmt.Printf("Received book response with status code: %d\n", responseBook.StatusCode)

                    if isCodeFail(responseBook.StatusCode) {
                        fmt.Printf("Book request failed with status code: %d\n", responseBook.StatusCode)
                        continue
                    }

                    responseBookBody, err := io.ReadAll(responseBook.Body)
                    if err != nil {
                        fmt.Printf("Error reading book response body: %v\n", err)
                        continue
                    }
                    fmt.Printf("Book response body: %s\n", string(responseBookBody))

                    var bookTopLevelMap map[string]interface{}
                    err = json.Unmarshal(responseBookBody, &bookTopLevelMap)
                    if err != nil {
                        fmt.Printf("Error unmarshaling book response JSON: %v\n", err)
                        continue
                    }

                    // Check if booking was successful
                    if _, ok := bookTopLevelMap["reservation_id"]; ok {
                        fmt.Println("Booking confirmed successfully")
                        resp := api.ReserveResponse{
                            ReservationTime: currentTime,
                        }
                        return &resp, nil
                    } else {
                        fmt.Println("Booking response does not contain confirmation")
                        fmt.Printf("Book response JSON: %v\n", bookTopLevelMap)
                        continue
                    }
                }
            }
        }
    }

    // If no table was found after all iterations
    fmt.Println("No available tables found for the given parameters")
    return nil, api.ErrNoTable
}


/*
Name: AuthMinExpire 
Type: API Func 
Purpose: Resy implementation of the AuthMinExpire api func.
The largest minimum validity time is 6 days.
*/
func (a *API) AuthMinExpire() (time.Duration) {
    /* 6 days */
    var d time.Duration = time.Hour * 24 * 6
    return d
}

//func (a *API) Cancel(params api.CancelParam) (*api.CancelResponse, error) {
//    cancelUrl := `https://api.resy.com/3/cancel` 
//    resyToken := url.QueryEscape(params.ResyToken)
//    requestBodyStr := "resy_token=" + resyToken
//    request, err := http.NewRequest("POST", cancelUrl, bytes.NewBuffer([]byte(requestBodyStr)))
//    if err != nil {
//        return nil, err
//    }
//    
//    request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
//    request.Header.Set("Authorization", `ResyAPI api_key="` + a.APIKey + `"`)
//    request.Header.Set("X-Resy-Auth-Token", params.AuthToken)
//    request.Header.Set("X-Resy-Universal-Auth-Token", params.AuthToken)
//    request.Header.Set("Referer", "https://resy.com/")
//    request.Header.Set("Origin", "https://resy.com")
//
//
//    client := &http.Client{}
//    response, err := client.Do(request)
//    if err != nil {
//        return nil, err
//    }
//
//    if isCodeFail(response.StatusCode) {
//        return nil, api.ErrNetwork
//    }
//
//    responseBody, err := io.ReadAll(response.Body)
//    if err != nil {
//        return nil, err 
//    }
//
//    defer response.Body.Close()
//    var jsonTopLevelMap map[string]interface{}
//    err = json.Unmarshal(responseBody, &jsonTopLevelMap)
//    if err != nil {
//        return nil, err
//    }
//
//    jsonPaymentMap := jsonTopLevelMap["payment"].(map[string]interface{})
//    jsonTransactionMap := jsonPaymentMap["transaction"].(map[string]interface{})
//    refund := jsonTransactionMap["refund"].(int) == 1
//    return &api.CancelResponse{Refund: refund}, nil
//}
//
