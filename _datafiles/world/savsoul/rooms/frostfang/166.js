
function onCommand_vault(rest, user, room) {

    var presentMob = null;

    var roomMobs = room.GetMobs();
    for (i = 0; i < roomMobs.length; i++) {
        var mob = roomMobs[i];
        mobName = mob.GetCharacterName(false);
        if ( mobName.indexOf("guard") !== -1 ) {
            presentMob = mob;
            break;
        }
    }

    if ( user.HasBuffFlag("hidden") ) {
        return false;
    }
    
    if ( presentMob != null ) {
        SendUserMessage(user.UserId(), presentMob.GetCharacterName(true) + " blocks you from entering the vault.");
        SendRoomMessage(room.RoomId(), presentMob.GetCharacterName(true) + " blocks " + user.GetCharacterName(true) + " from entering the vault.", user.UserId());
        presentMob.Command(`sayto ` + user.ShorthandId() + ` not on my watch, pal.`, 1.0);
        return true;
    }

    return false;
}

