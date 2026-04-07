import math
import os
import re
import shutil
from urllib.parse import parse_qs, urlparse

import lark_oapi as lark
import pandas
import requests
from lark_oapi.api.sheets.v3 import (
    GetSpreadsheetRequest,
    GetSpreadsheetResponse,
    QuerySpreadsheetSheetRequest,
    QuerySpreadsheetSheetResponse,
)

########################################## 配置区域 ##########################################
APP_ID = "cli_a949ffac3d789cb6"
APP_SCRET = "REGgmdiT9oBcfNE60jjehhLJ6tWBOm2P"
SOURCE_URLS = [
    "https://chillyroom.feishu.cn/sheets/A4Xcs9FqUhU6MTt8pxdc1GEinIc",
    "https://chillyroom.feishu.cn/sheets/HdWNsWbUdhxBBztJE2DcYLQZnIb"
]
OUTPUT_DIR = "./Config"
CSV_COPY_TARGETS = {
    "Data_": ["../IAAClient/Assets/Configs/", "../IAAServer/svr_game/"],
    "Localization_": ["../IAAClient/Assets/Configs/"],
}
########################################## 配置区域 ##########################################

def raise_for_response(response, action):
    try:
        payload = response.json()
    except ValueError:
        payload = response.text

    if response.status_code >= 400:
        raise RuntimeError(
            f"{action} failed: http={response.status_code}, body={payload}"
        )

    return payload


def get_tenant_access_token():
    response = requests.post(
        "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
        json={
            "app_id": APP_ID,
            "app_secret": APP_SCRET,
        },
    )
    payload = raise_for_response(response, "get tenant_access_token")
    if payload.get("code") not in (None, 0):
        raise RuntimeError(f"failed to get tenant_access_token: {payload}")

    token = payload.get("tenant_access_token")
    if not token:
        raise RuntimeError(f"tenant_access_token missing: {payload}")

    return token


tenant_access_token = get_tenant_access_token()

client = (
    lark.Client.builder()
    .app_id(APP_ID)
    .app_secret(APP_SCRET)
    .log_level(lark.LogLevel.INFO)
    .build()
)


def build_headers():
    return {
        "Authorization": f"Bearer {tenant_access_token}",
        "Content-Type": "application/json; charset=utf-8",
    }


def parse_source_url(url):
    parsed = urlparse(url)
    path_parts = [part for part in parsed.path.split("/") if part]
    if len(path_parts) < 2:
        raise ValueError(f"unsupported SOURCE_URL: {url}")

    source_type = path_parts[-2]
    spreadsheet_token = path_parts[-1]
    sheet_id = parse_qs(parsed.query).get("sheet", [None])[0]

    if source_type != "sheets":
        raise ValueError(f"SOURCE_URL must be a Feishu sheets URL: {url}")

    return spreadsheet_token, sheet_id


def grab_spread_file_info(token):
    request: GetSpreadsheetRequest = (
        GetSpreadsheetRequest.builder()
        .spreadsheet_token(token)
        .user_id_type("open_id")
        .build()
    )
    response: GetSpreadsheetResponse = client.sheets.v3.spreadsheet.get(request)

    if not response.success():
        raise RuntimeError(f"get spreadsheet failed: {response.msg}")

    return response.data


def grab_spreadsheet_sheets_info(token):
    request: QuerySpreadsheetSheetRequest = (
        QuerySpreadsheetSheetRequest.builder().spreadsheet_token(token).build()
    )
    response: QuerySpreadsheetSheetResponse = client.sheets.v3.spreadsheet_sheet.query(
        request
    )

    if not response.success():
        raise RuntimeError(f"query spreadsheet sheets failed: {response.msg}")

    return response.data


def num_to_letter(num):
    if num > 26:
        first = num_to_letter(math.floor((num - 1) / 26))
        second = num_to_letter(num % 26)
        if second == "@":
            second = "Z"
        return first + second
    return chr(num + ord("A") - 1)


def sanitize_filename(name):
    return re.sub(r'[\\/:*?"<>|]+', "_", name).strip()


def is_effectively_empty(value):
    if value is None:
        return True

    text = str(value)
    text = re.sub(r"[\s\u200b\u200c\u200d\ufeff\u2060]+", "", text)
    return text == ""


def fetch_sheet_values(spreadsheet_token, sheet_id, column_count, row_count):
    last_column = num_to_letter(column_count)
    value_range = f"{sheet_id}!A1:{last_column}{row_count}"
    response = requests.get(
        f"https://open.feishu.cn/open-apis/sheets/v2/spreadsheets/{spreadsheet_token}/values_batch_get",
        params={
            "ranges": value_range,
            "valueRenderOption": "ToString",
            "dateTimeRenderOption": "FormattedString",
        },
        headers=build_headers(),
    )
    payload = raise_for_response(response, "fetch sheet values")
    if payload.get("code") not in (None, 0):
        raise RuntimeError(f"failed to fetch sheet values: {payload}")

    value_ranges = payload.get("data", {}).get("valueRanges", [])
    if not value_ranges:
        return []

    return value_ranges[0].get("values", [])


def rows_to_dataframe(rows):
    if not rows:
        return pandas.DataFrame()

    headers = rows[0]
    kept_indexes = []
    normalized_headers = []
    for index, value in enumerate(headers):
        if is_effectively_empty(value):
            continue

        title = str(value).strip()
        kept_indexes.append(index)
        normalized_headers.append(title)

    body_rows = []
    for row in rows[1:]:
        first_cell = row[0] if row else ""
        if is_effectively_empty(first_cell):
            continue

        filtered_row = []
        for index in kept_indexes:
            filtered_row.append(row[index] if index < len(row) else "")
        body_rows.append(filtered_row)

    return pandas.DataFrame(body_rows, columns=normalized_headers)


def export_sheet_files(spreadsheet_name, sheet):
    rows = fetch_sheet_values(
        spreadsheet_name["token"],
        sheet.sheet_id,
        sheet.grid_properties.column_count,
        sheet.grid_properties.row_count,
    )
    dataframe = rows_to_dataframe(rows)

    safe_spreadsheet_name = sanitize_filename(spreadsheet_name["title"])
    safe_sheet_name = sanitize_filename(sheet.title)
    base_name = f"{safe_spreadsheet_name}_{safe_sheet_name}"
    csv_path = os.path.join(OUTPUT_DIR, f"{base_name}.csv")

    dataframe.to_csv(csv_path, index=False, encoding="utf-8-sig")
    print(f"csv: {csv_path}")

    return sheet.title, dataframe, csv_path


def export_sheet(spreadsheet_token, target_sheet_id):
    file_data = grab_spread_file_info(spreadsheet_token)
    sheet_data = grab_spreadsheet_sheets_info(spreadsheet_token)

    if not sheet_data.sheets:
        raise RuntimeError(f"no sheets found in spreadsheet: {spreadsheet_token}")

    target_sheets = []
    if target_sheet_id:
        for sheet in sheet_data.sheets:
            if sheet.sheet_id == target_sheet_id:
                target_sheets = [sheet]
                break
    else:
        target_sheets = list(sheet_data.sheets)

    if not target_sheets:
        raise RuntimeError(f"sheet id not found in spreadsheet: {target_sheet_id}")

    spreadsheet_info = {
        "token": spreadsheet_token,
        "title": file_data.spreadsheet.title,
    }
    xlsx_path = os.path.join(
        OUTPUT_DIR, f"{sanitize_filename(file_data.spreadsheet.title)}.xlsx"
    )
    exported_csv_paths = []

    with pandas.ExcelWriter(xlsx_path, engine="xlsxwriter") as writer:
        for sheet in target_sheets:
            _, dataframe, csv_path = export_sheet_files(spreadsheet_info, sheet)
            safe_sheet_name = sanitize_filename(sheet.title)
            dataframe.to_excel(
                writer,
                sheet_name=safe_sheet_name[:31] or "Sheet1",
                index=False,
            )
            exported_csv_paths.append(csv_path)

    print(f"downloaded spreadsheet: {file_data.spreadsheet.title}")
    print(f"xlsx: {xlsx_path}")
    return exported_csv_paths


def resolve_copy_destination(csv_path, target_path):
    normalized_target = os.path.normpath(target_path)
    if normalized_target.lower().endswith(".csv"):
        return normalized_target

    return os.path.join(normalized_target, os.path.basename(csv_path))


def distribute_csv_files(csv_paths):
    copied_count = 0
    for csv_path in csv_paths:
        file_name = os.path.basename(csv_path)
        if not file_name.lower().endswith(".csv"):
            continue

        for prefix, target_paths in CSV_COPY_TARGETS.items():
            if not file_name.startswith(prefix):
                continue

            for target_path in target_paths:
                destination = resolve_copy_destination(csv_path, target_path)
                destination_dir = os.path.dirname(destination)
                if destination_dir:
                    os.makedirs(destination_dir, exist_ok=True)

                shutil.copy2(csv_path, destination)
                copied_count += 1
                print(f"copied: {csv_path} -> {destination}")

    return copied_count


def export_urls(source_urls):
    if not source_urls:
        raise RuntimeError("SOURCE_URLS is empty")

    exported_count = 0
    exported_csv_paths = []
    skipped_urls = []
    for source_url in source_urls:
        try:
            spreadsheet_token, sheet_id = parse_source_url(source_url)
            exported_csv_paths.extend(export_sheet(spreadsheet_token, sheet_id))
            exported_count += 1
        except Exception as error:
            skipped_urls.append((source_url, str(error)))

    if exported_count == 0:
        raise RuntimeError("no spreadsheets exported from SOURCE_URLS")

    print(f"exported spreadsheets: {exported_count}")
    for source_url, error in skipped_urls:
        print(f"skipped: {source_url} -> {error}")

    return exported_csv_paths


def prepare_output_dir():
    os.makedirs(OUTPUT_DIR, exist_ok=True)


def main():
    prepare_output_dir()
    exported_csv_paths = export_urls(SOURCE_URLS)
    copied_count = distribute_csv_files(exported_csv_paths)
    print(f"copied csv files: {copied_count}")


if __name__ == "__main__":
    main()
