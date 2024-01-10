# DrovaNotifierV2
Программа для запуска на станциях в сервисе drova.io, оповещает о начале и окончании сессии через telegram. 

Использовался go1.21.3

Запуск

1. Установите Golang https://go.dev/
2. Создайте нового(или используем старого) бота в Telegram с помощью BotFather https://telegram.me/BotFather
3. Скопируйте все файлы на свой локальный компьютер и распакуйте
4. Если требуется, переименовываем ПК в винде, оно будет отправляться в чат как имя станции
5. В файле config.txt меняем значения на свои. Можно обойтись без использования файла config.txt. До компиляции в файле config.go  вписать свои значения
6. Для компиляции используем copilate и получаем исполняемый файл DrovaNotifierV2.exe
7. Закидываем исполняемый файл и отредактированный config.txt на станцию и запускаем

Для мониторинга температур, требуется запущенный локальный веб сервер LibreHardwareMonitor.

При необходимости добавляем в автозагрузку или планировщик задач для автостарта

Для выяснения ID чата или ID пользователя можно воспользоваться ботом https://t.me/username_to_id_bot

Команды которые можно применять в боте. Вначале / убран чтобы было проще скормить в BotFather для использования готовых команд, ST3 - имя вашейго ПК, на котором крутится бот
<BR><BR>
/start - выводит список доступных комманд
rebootST3 - перезагрузить St3<BR>
delayrebootST3 - отложенная перезагрузка St3<BR>
visibleST3 - скрыть St3<BR>
invisibleST3 - скрыть St3<BR>
status - статус серверов<BR>
temp - температуры<BR>
drovastartST3 - старт Streaming Service St3<BR>
drovastopST3 - стоп Streaming Service St3<BR>
