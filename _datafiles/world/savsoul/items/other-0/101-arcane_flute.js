
const MUSIC_DESCRIPTIONS = [
    "a silvery trill dances on the breeze. ♪♪ ♫",
    "a cascade of notes unravels into open air. ♫ ♪♪",
    "a soft melody fills the air. ♪♫ ♪",
];

const RAT_MOB_IDS = [1, 12];

function onCommand_play(user, item, room) {

    var randomPhrase = MUSIC_DESCRIPTIONS[UtilDiceRoll(1, MUSIC_DESCRIPTIONS.length)-1];

    if ( UtilIsDay() ) {
        SendUserMessage(user.UserId(), "You attempt to play the flute, but only succeed in producing a shrill noise");
        SendRoomMessage(room.RoomId(), user.GetCharacterName(true)+" attempts to play their <ansi fg=\"item\">"+item.Name(true)+"</ansi> and a horrible, shrill sound fills the air.", user.UserId());   
        
        return true;
    } 

    SendUserMessage(user.UserId(), "You surprisingly find yourself able to play the flute effortlessly, and "+randomPhrase);
    SendRoomMessage(room.RoomId(), user.GetCharacterName(true)+" plays their <ansi fg=\"item\">"+item.Name(true)+"</ansi> and "+randomPhrase, user.UserId());   

    // NOTE: This is not charming the mob. This is a special pacify and force follow.
    //       The rats will not follow the behavior of charmed mobs.
    for( var i in RAT_MOB_IDS ) {
        var ratMobs = room.GetMobs(RAT_MOB_IDS[i]);
        for ( var j in ratMobs ) {
            ratMobs[j].Command(`break`); // Break off any combat
            ratMobs[j].Command(`follow ` + user.ShorthandId() + ` sunrise`); // follow whoever played the flute until sunrise
            ratMobs[j].ChangeAlignment(user.GetAlignment()); // Set alignment to the flute player
        }
    }


    return true;
}

