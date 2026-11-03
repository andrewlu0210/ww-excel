import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

// Exercise the actual editor/controller, backend diff, cache and canvas together.
export async function richTextWorkbookSelfTest(driver: ReturnType<typeof Sheet>): Promise<{ markdown: string; failed: number }> {
  const checks: { name: string; ok: boolean; detail: string }[] = [];
  const check = (name: string, ok: boolean, detail = '') => checks.push({ name, ok, detail });
  const idle = async () => {
    await driver.whenIdle();
    await document.fonts.ready;
    await new Promise((resolve) => setTimeout(resolve, 200));
  };
  const verify = async (label: string, body: string) => {
    await idle();
    const blocks = await sheetApi.getBlocks(0, [{ row: 0, col: 0 }], 0);
    const cell = blocks.blocks?.[0]?.cells?.find((c) => c.r === 0 && c.c === 0);
    const runs = cell?.runs ?? [];
    check(`${label}：後端保留紅色粗體標題與黑色內文`,
      cell?.v === `標題${body}` && runs.length === 2 &&
      runs[0].text === '標題' && runs[0].font.color === 'FF0000' && runs[0].font.bold === true &&
      runs[1].text === body && runs[1].font.color === '000000' && !runs[1].font.bold,
      JSON.stringify(cell));
    const grid = driver.getGrid();
    const cached = grid?.cache.cell(0, 0);
    check(`${label}：前端快取保留分段格式`, JSON.stringify(cached?.runs) === JSON.stringify(runs), JSON.stringify(cached));
    const canvas = document.querySelector<HTMLCanvasElement>('.grid-host > canvas');
    const rect = grid?.cellRect({ row: 0, col: 0 });
    let red = 0;
    if (canvas && rect) {
      const dpr = window.devicePixelRatio || 1;
      const pixels = canvas.getContext('2d')!.getImageData(
        Math.ceil(rect.x * dpr), Math.ceil(rect.y * dpr), Math.floor(rect.w * dpr), Math.floor(rect.h * dpr),
      ).data;
      for (let i = 0; i < pixels.length; i += 4) {
        if (pixels[i] > 150 && pixels[i + 1] < 100 && pixels[i + 2] < 100 && pixels[i + 3] > 200) red++;
      }
    }
    check(`${label}：Canvas 實際繪製紅色標題`, red > 20, `${red} red pixels`);
  };
  const replaceBody = async (body: string, bar = false) => {
    driver.clickCell({ row: 0, col: 0 }, 'down');
    driver.clickCell({ row: 0, col: 0 }, 'up');
    driver.clickCell({ row: 0, col: 0 }, 'dbl');
    await idle();
    check('雙擊進入 A1 編輯', driver.getEditing()?.pos.row === 0 && driver.getEditing()?.pos.col === 0);
    if (bar) {
      driver.focusBar();
      await idle();
    }
    driver.selectEditText(2, driver.getBarText().length);
    driver.typeText(body);
    driver.pressKey('Enter');
    await idle();
  };

  await verify('匯入', '內文');
  await benchApi.snapshot('richtext-before', driver.snapshotWithObjects());
  await replaceBody('說明');
  await verify('雙擊修改內文', '說明');
  await benchApi.snapshot('richtext-after-cell-edit', driver.snapshotWithObjects());
  await driver.runMenu('undo');
  await verify('復原', '內文');
  await driver.runMenu('redo');
  await verify('重做', '說明');
  await replaceBody('補充🙂', true);
  await verify('公式列修改內文', '補充🙂');
  for (const name of ['richtext-edited.xlsx', 'richtext-edited.wwx', 'richtext-from-native.xlsx']) {
    const path = await benchApi.saveCopy(name);
    await documentApi.openPath(path);
    await verify(`另存重開 ${name}`, '補充🙂');
  }
  await benchApi.snapshot('richtext-after-roundtrip', driver.snapshotWithObjects());
  // Use real contenteditable DOM, toolbar controls and native input events.
  const cellAt=async(row:number,col:number)=>(await sheetApi.getBlocks(0,[{row:0,col:0}],0)).blocks?.[0]?.cells?.find(c=>c.r===row && c.c===col);
  const open=async(row:number,col:number,formula=false)=>{
    driver.clickCell({row,col},'down');driver.clickCell({row,col},'up');await idle();
    driver.clickCell({row,col},'dbl');await idle();if(formula){driver.focusBar();await idle();}
  };
  const control=()=>document.activeElement?.classList.contains('formula') ? document.activeElement as HTMLDivElement : document.querySelector<HTMLDivElement>('.cell-editor')!;
  const choose=async(selector:string,value:string)=>{
    const select=document.querySelector<HTMLSelectElement>(selector)!;
    select.focus();select.value=value;select.dispatchEvent(new Event('change',{bubbles:true}));await idle();
  };
  const click=async(selector:string)=>{document.querySelector<HTMLButtonElement>(selector)!.click();await idle();};
  const nativeInsert=(text:string)=>document.execCommand('insertText',false,text);
  const textOf=(row:number,col:number)=>cellAt(row,col).then(c=>c?.v);
  const initial='紅色標題黑色內文🙂';
  await sheetApi.setCell(0,2,1,initial);await open(2,1);
  driver.selectEditText(0,4);
  await click('.toolbar .b');
  await choose('.toolbar select.font','Arial');
  await choose('.toolbar select.size','20');
  await click('.toolbar button[title="選擇字型色彩"]');
  await click('.picker button[title="#FF0000"]');
  let spans=[...control().querySelectorAll('span')];
  check('工具列只格式化選取的紅色標題',spans[0]?.textContent==='紅色標題' && getComputedStyle(spans[0]).color==='rgb(255, 0, 0)' && getComputedStyle(spans[0]).fontWeight==='700' && Math.abs(parseFloat(getComputedStyle(spans[0]).fontSize)-20*4/3)<.1);
  check('格式套用後保留文字選取與編輯狀態',!!driver.getEditing() && window.getSelection()?.toString()==='紅色標題');
  check('尚未提交時後端內容與格式不變',!(await cellAt(2,1))?.runs);
  document.querySelector<HTMLSelectElement>('.toolbar select.font')!.focus();
  await driver.runMenu('undo');await idle();spans=[...control().querySelectorAll('span')];
  check('編輯中復原顏色仍保留粗體與字級',document.activeElement===control() && !!spans[0] && getComputedStyle(spans[0]).color!=='rgb(255, 0, 0)' && getComputedStyle(spans[0]).fontWeight==='700');
  await driver.runMenu('redo');await idle();
  driver.selectEditText(2,6);await driver.applyFormat({underline:true,strike:true});await idle();
  spans=[...control().querySelectorAll('span')];
  check('跨區段套用底線與刪除線不覆蓋原色',spans.some(s=>s.textContent==='標題' && s.style.color==='rgb(255, 0, 0)' && s.style.textDecoration.includes('underline') && s.style.textDecoration.includes('line-through')) && spans.some(s=>s.textContent==='黑色' && s.style.color!== 'rgb(255, 0, 0)' && s.style.textDecoration.includes('underline')));
  driver.selectEditText(4,initial.length);await driver.applyFormat({fontColor:'000000',bold:false,italic:true,underline:false,strike:false});await idle();
  driver.selectEditText(4,4);nativeInsert('🙂');await idle();
  check('區段邊界插入繼承左側格式',control().textContent==='紅色標題🙂黑色內文🙂' && [...control().querySelectorAll('span')].some(s=>s.textContent?.includes('🙂') && s.style.color==='rgb(255, 0, 0)'));
  // Simulate a full composition lifecycle without rewriting its live DOM.
  driver.selectEditText(4,6);const composing=control();
  composing.dispatchEvent(new CompositionEvent('compositionstart',{bubbles:true,data:''}));
  nativeInsert('中');
  driver.pressKey('Enter',{isComposing:true});
  check('中文組字期間 Enter 不提交',!!driver.getEditing() && (await textOf(2,1))===initial);
  driver.selectEditText(4,5);nativeInsert('中文');
  composing.dispatchEvent(new CompositionEvent('compositionend',{bubbles:true,data:'中文'}));await idle();
  check('中文組字完成保留區段格式',control().textContent==='紅色標題中文黑色內文🙂' && [...control().querySelectorAll('span')].some(s=>s.textContent?.includes('中文') && s.style.color==='rgb(255, 0, 0)'));
  await new Promise(r=>setTimeout(r,160));await driver.runMenu('undo');await idle();
  check('中文組字為一次復原',control().textContent==='紅色標題🙂黑色內文🙂');
  await driver.runMenu('redo');await idle();
  check('中文組字重做恢復內容與格式',control().textContent==='紅色標題中文黑色內文🙂');
  driver.pressKey('Escape');await idle();
  check('取消同時捨棄文字與格式修改',(await textOf(2,1))===initial && !(await cellAt(2,1))?.runs);

  await open(2,1,true);driver.selectEditText(0,4);
  await driver.applyFormat({fontColor:'FF0000',bold:true,fontSize:20,fontName:'Arial'});await idle();
  check('公式列局部格式可見且保持編輯',!!driver.getEditing() && document.activeElement?.classList.contains('formula')===true && control().querySelector('span')?.textContent==='紅色標題' && control().querySelector('span')?.style.color==='rgb(255, 0, 0)');
  driver.selectEditText(initial.length,initial.length);nativeInsert('補充');await idle();
  driver.pressKey('Enter');await idle();
  const committed=await cellAt(2,1),savedRuns=committed?.runs;
  check('大字級自動增加列高避免裁切',(driver.getGrid()?.rowHeight(2) ?? 0)>=37);
  check('提交文字與分段格式為單一變更',committed?.v===initial+'補充' && savedRuns?.[0].text==='紅色標題' && savedRuns[0].font.bold===true && savedRuns[0].font.size===20 && savedRuns[0].font.color==='FF0000' && !savedRuns[1].font.bold);
  await new Promise(r=>setTimeout(r,160));await driver.runMenu('undo');await idle();
  check('提交後一次復原還原文字與格式',(await textOf(2,1))===initial && !(await cellAt(2,1))?.runs);
  check('列高與文字格式共用一次復原',driver.getGrid()?.rowHeight(2)===20);
  await driver.runMenu('redo');await idle();
  check('提交後重做還原文字與格式',JSON.stringify((await cellAt(2,1))?.runs)===JSON.stringify(savedRuns));
  driver.goTo({row:2,col:1});await idle();await benchApi.snapshot('richtext-selection-format',driver.snapshotWithObjects());
  for(const name of ['richtext-selection.xlsx','richtext-selection.wwx','richtext-selection-native.xlsx']){
    const path=await benchApi.saveCopy(name);await documentApi.openPath(path);await idle();
    check(`局部格式另存重開 ${name}`,JSON.stringify((await cellAt(2,1))?.runs)===JSON.stringify(savedRuns));
  }
  await sheetApi.setCell(0,4,1,"'=SUM(A1)");await open(4,1);driver.selectEditText(1,5);
  await driver.applyFormat({fontColor:'0070C0',underline:true});driver.pressKey('Enter');await idle();
  check('文字公式的跳脫符號不進入儲存內容',(await textOf(4,1))==='=SUM(A1)' && !!(await cellAt(4,1))?.runs?.some(r=>r.text==='=SUM' && r.font.underline));
  await open(2,1);driver.selectEditText(4,initial.length+2);
  const clipboard=new DataTransfer();clipboard.setData('text/plain','貼上\n內文');clipboard.setData('text/html','<img src=x onerror=alert(1)><b>錯誤</b>');
  control().dispatchEvent(new ClipboardEvent('paste',{bubbles:true,cancelable:true,clipboardData:clipboard}));await idle();
  check('貼上只接受純文字且保留換行與標題',control().textContent==='紅色標題貼上\n內文' && !control().querySelector('img,b'));
  driver.pressKey('Enter');await idle();
  check('多行提交保留原標題格式',(await cellAt(2,1))?.runs?.[0].text==='紅色標題' && (await textOf(2,1))==='紅色標題貼上\n內文');
  await sheetApi.setCell(0,6,1,'單格確認');await open(6,1);driver.selectEditText(0,2);
  await driver.applyFormat({fontColor:'0070C0',bold:true});driver.pressKey('Enter',{metaKey:true});await idle();
  check('單格 ⌘Enter 提交保留局部格式',(await cellAt(6,1))?.runs?.[0].text==='單格' && (await cellAt(6,1))?.runs?.[0].font.color==='0070C0' && !driver.getEditing());
  await open(6,1);driver.selectEditText(0,4);nativeInsert('123');driver.pressKey('Enter');await idle();
  check('富文字改為數值仍使用一般輸入轉換',(await cellAt(6,1))?.n===true && (await textOf(6,1))==='123' && !(await cellAt(6,1))?.runs);
  await open(0,0);driver.selectEditText(0,driver.getBarText().length);document.execCommand('delete');driver.pressKey('Enter');await idle();
  check('刪除富文字全部內容可提交空白',!driver.getEditing() && !(await cellAt(0,0))?.v);
  const failed = checks.filter((c) => !c.ok).length;
  return {
    failed,
    markdown: ['# 富文字桌面編輯回歸測試', '', `${checks.length - failed} / ${checks.length} 通過`, '',
      ...checks.map((c) => `${c.ok ? '✅' : '❌'} ${c.name}: ${c.detail}`), ''].join('\n'),
  };
}
