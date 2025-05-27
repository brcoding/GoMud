
function onCommand_out(rest, user, room) {
    
    var mobs = room.GetMobs();
    if ( mobs.length > 0 ) {
        SendUserMessage(user.UserId(), "The way out is block by denizens of the cave.");
        return true;
    }

    return false;
}
